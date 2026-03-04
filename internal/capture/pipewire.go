package capture

/*
#cgo pkg-config: libpipewire-0.3 libdrm
#cgo CFLAGS: -Wno-deprecated-declarations

#include <pipewire/pipewire.h>
#include <spa/param/video/format-utils.h>
#include <spa/param/video/raw.h>
#include <spa/debug/types.h>
#include <spa/param/video/type-info.h>
#include <spa/buffer/buffer.h>

#include <xf86drm.h>
#include <xf86drmMode.h>
#include <drm/drm_fourcc.h>

#include <dirent.h>
#include <fcntl.h>
#include <stdlib.h>
#include <string.h>
#include <sys/mman.h>

// Maximum number of DRM modifiers we'll collect.
#define MAX_MODIFIERS 64

// drm_modifier_list holds the result of a DRM modifier query.
struct drm_modifier_list {
	int64_t  mods[MAX_MODIFIERS];
	uint32_t count;
};

// has_modifier returns 1 if the list already contains the given modifier.
static int has_modifier(struct drm_modifier_list *list, int64_t mod)
{
	for (uint32_t i = 0; i < list->count; i++) {
		if (list->mods[i] == mod)
			return 1;
	}
	return 0;
}

// add_modifier appends a modifier if not already present and not at capacity.
static void add_modifier(struct drm_modifier_list *list, int64_t mod)
{
	if (list->count >= MAX_MODIFIERS)
		return;
	if (has_modifier(list, mod))
		return;
	list->mods[list->count++] = mod;
}

// query_drm_modifiers collects all DRM modifiers the GPU supports for
// DRM_FORMAT_XRGB8888 (= SPA_VIDEO_FORMAT_BGRx). Falls back to
// {LINEAR, INVALID} if the query fails.
static struct drm_modifier_list query_drm_modifiers(void)
{
	struct drm_modifier_list list = {0};

	// Find the first available card (primary) node in /dev/dri/.
	// Render nodes don't support KMS plane queries.
	int card_fd = -1;
	DIR *dir = opendir("/dev/dri");
	if (dir == NULL)
		goto fallback;

	struct dirent *ent;
	while ((ent = readdir(dir)) != NULL) {
		if (strncmp(ent->d_name, "card", 4) != 0)
			continue;
		char path[64];
		snprintf(path, sizeof(path), "/dev/dri/%s", ent->d_name);
		card_fd = open(path, O_RDWR | O_CLOEXEC);
		if (card_fd >= 0)
			break;
	}
	closedir(dir);

	if (card_fd < 0)
		goto fallback;

	// Enable universal planes so we can see all planes.
	if (drmSetClientCap(card_fd, DRM_CLIENT_CAP_UNIVERSAL_PLANES, 1) != 0) {
		close(card_fd);
		goto fallback;
	}

	drmModePlaneResPtr plane_res = drmModeGetPlaneResources(card_fd);
	if (plane_res == NULL) {
		close(card_fd);
		goto fallback;
	}

	for (uint32_t i = 0; i < plane_res->count_planes; i++) {
		drmModeObjectPropertiesPtr props = drmModeObjectGetProperties(
			card_fd, plane_res->planes[i], DRM_MODE_OBJECT_PLANE);
		if (props == NULL)
			continue;

		for (uint32_t j = 0; j < props->count_props; j++) {
			drmModePropertyPtr prop =
				drmModeGetProperty(card_fd, props->props[j]);
			if (prop == NULL)
				continue;

			if (strcmp(prop->name, "IN_FORMATS") == 0) {
				drmModePropertyBlobPtr blob =
					drmModeGetPropertyBlob(
						card_fd, props->prop_values[j]);
				if (blob != NULL) {
					drmModeFormatModifierIterator iter = {0};
					while (drmModeFormatModifierBlobIterNext(
							blob, &iter)) {
						if (iter.fmt == DRM_FORMAT_XRGB8888)
							add_modifier(&list,
								(int64_t)iter.mod);
					}
					drmModeFreePropertyBlob(blob);
				}
			}
			drmModeFreeProperty(prop);
		}
		drmModeFreeObjectProperties(props);
	}

	drmModeFreePlaneResources(plane_res);
	close(card_fd);

	// Ensure LINEAR and INVALID are always in the list.
	add_modifier(&list, (int64_t)DRM_FORMAT_MOD_LINEAR);
	add_modifier(&list, (int64_t)DRM_FORMAT_MOD_INVALID);

	if (list.count > 0)
		return list;

fallback:
	list.count = 0;
	list.mods[list.count++] = (int64_t)DRM_FORMAT_MOD_LINEAR;
	list.mods[list.count++] = (int64_t)DRM_FORMAT_MOD_INVALID;
	return list;
}

// capture_result holds the output of a single-frame PipeWire capture.
struct capture_result {
	void     *data;    // malloc'd pixel buffer (caller frees)
	uint32_t  width;
	uint32_t  height;
	uint32_t  stride;
	uint32_t  format;  // SPA_VIDEO_FORMAT_*
	char     *error;   // malloc'd error string or NULL (caller frees)
};

// pw_capture_state tracks internal state for the capture operation.
struct pw_capture_state {
	struct pw_main_loop    *loop;
	struct pw_stream       *stream;
	struct spa_video_info   format;
	struct capture_result  *result;
	int                     format_ready;
	int                     is_dmabuf;
};

// copy_frame copies pixel data from the source buffer to a malloc'd output
// buffer, handling stride differences. src may come from mmap or direct data.
static void copy_frame(struct pw_capture_state *state, uint8_t *src,
		       uint32_t src_stride, uint32_t src_size)
{
	uint32_t width  = state->format.info.raw.size.width;
	uint32_t height = state->format.info.raw.size.height;
	uint32_t row_bytes = width * 4;

	void *out = malloc(row_bytes * height);
	if (out == NULL) {
		state->result->error = strdup("malloc failed for frame buffer");
		return;
	}

	uint8_t *dst = (uint8_t *)out;

	// Guard against reading past the source buffer.
	uint32_t copy_rows = height;
	if (src_stride * height > src_size && src_size > 0)
		copy_rows = src_size / src_stride;

	for (uint32_t row = 0; row < copy_rows; row++) {
		memcpy(dst + row * row_bytes, src + row * src_stride, row_bytes);
	}

	state->result->data   = out;
	state->result->width  = width;
	state->result->height = copy_rows;
	state->result->stride = row_bytes;
	state->result->format = state->format.info.raw.format;
}

static void on_process(void *userdata)
{
	struct pw_capture_state *state = userdata;
	struct pw_buffer *b;
	struct spa_buffer *buf;

	if ((b = pw_stream_dequeue_buffer(state->stream)) == NULL)
		return;

	buf = b->buffer;

	if (!state->format_ready) {
		pw_stream_queue_buffer(state->stream, b);
		return;
	}

	uint32_t stride = buf->datas[0].chunk->stride;
	uint32_t size   = buf->datas[0].chunk->size;

	if (stride == 0)
		stride = state->format.info.raw.size.width * 4;

	if (buf->datas[0].data != NULL) {
		// MemPtr or MemFd (auto-mapped by PW_STREAM_FLAG_MAP_BUFFERS).
		copy_frame(state, (uint8_t *)buf->datas[0].data, stride, size);
	} else if (buf->datas[0].type == SPA_DATA_DmaBuf ||
		   buf->datas[0].type == SPA_DATA_MemFd) {
		// DMA-BUF or MemFd not auto-mapped: mmap the fd manually.
		uint32_t map_size = buf->datas[0].maxsize;
		if (map_size == 0)
			map_size = stride * state->format.info.raw.size.height;

		void *mapped = mmap(NULL, map_size, PROT_READ, MAP_SHARED,
				    (int)buf->datas[0].fd,
				    buf->datas[0].mapoffset);
		if (mapped != MAP_FAILED) {
			copy_frame(state, (uint8_t *)mapped, stride, map_size);
			munmap(mapped, map_size);
		} else {
			state->result->error = strdup(
				"could not mmap PipeWire buffer");
		}
	} else {
		state->result->error = strdup(
			"PipeWire buffer has no accessible data");
	}

	pw_stream_queue_buffer(state->stream, b);
	pw_main_loop_quit(state->loop);
}

static void on_param_changed(void *userdata, uint32_t id,
			     const struct spa_pod *param)
{
	struct pw_capture_state *state = userdata;

	if (param == NULL || id != SPA_PARAM_Format)
		return;

	if (spa_format_parse(param,
		&state->format.media_type,
		&state->format.media_subtype) < 0)
		return;

	if (state->format.media_type != SPA_MEDIA_TYPE_video ||
	    state->format.media_subtype != SPA_MEDIA_SUBTYPE_raw)
		return;

	if (spa_format_video_raw_parse(param, &state->format.info.raw) < 0)
		return;

	// Check whether DMA-BUF was negotiated via the modifier flag.
	state->is_dmabuf =
		(state->format.info.raw.flags & SPA_VIDEO_FLAG_MODIFIER) != 0;

	// If the modifier still needs fixation, respond with a fixated
	// single-modifier EnumFormat pod. PipeWire will call us again with
	// the fully resolved format.
	if (state->format.info.raw.flags &
	    SPA_VIDEO_FLAG_MODIFIER_FIXATION_REQUIRED) {
		uint8_t fixbuf[1024];
		struct spa_pod_builder fb =
			SPA_POD_BUILDER_INIT(fixbuf, sizeof(fixbuf));
		struct spa_pod_frame ff[1];

		spa_pod_builder_push_object(&fb, &ff[0],
			SPA_TYPE_OBJECT_Format, SPA_PARAM_EnumFormat);
		spa_pod_builder_add(&fb,
			SPA_FORMAT_mediaType,
				SPA_POD_Id(SPA_MEDIA_TYPE_video),
			SPA_FORMAT_mediaSubtype,
				SPA_POD_Id(SPA_MEDIA_SUBTYPE_raw),
			SPA_FORMAT_VIDEO_format,
				SPA_POD_Id(state->format.info.raw.format),
			0);
		// Fixate: single modifier value, still MANDATORY.
		spa_pod_builder_prop(&fb, SPA_FORMAT_VIDEO_modifier,
			SPA_POD_PROP_FLAG_MANDATORY);
		spa_pod_builder_long(&fb,
			(int64_t)state->format.info.raw.modifier);
		spa_pod_builder_add(&fb,
			SPA_FORMAT_VIDEO_size, SPA_POD_Rectangle(
				&state->format.info.raw.size),
			SPA_FORMAT_VIDEO_framerate, SPA_POD_Fraction(
				&state->format.info.raw.framerate),
			0);
		const struct spa_pod *fixparam =
			spa_pod_builder_pop(&fb, &ff[0]);

		pw_stream_update_params(state->stream, &fixparam, 1);
		return;
	}

	// Format is fully resolved. Respond with SPA_PARAM_Buffers to
	// complete the buffer handshake.
	uint8_t buf[512];
	struct spa_pod_builder pb = SPA_POD_BUILDER_INIT(buf, sizeof(buf));

	int32_t data_type = state->is_dmabuf
		? (1 << SPA_DATA_DmaBuf)
		: ((1 << SPA_DATA_MemFd) | (1 << SPA_DATA_MemPtr));

	const struct spa_pod *bufparam = spa_pod_builder_add_object(&pb,
		SPA_TYPE_OBJECT_ParamBuffers, SPA_PARAM_Buffers,
		SPA_PARAM_BUFFERS_buffers,  SPA_POD_CHOICE_RANGE_Int(2, 1, 8),
		SPA_PARAM_BUFFERS_blocks,   SPA_POD_Int(1),
		SPA_PARAM_BUFFERS_dataType, SPA_POD_CHOICE_FLAGS_Int(data_type));

	pw_stream_update_params(state->stream, &bufparam, 1);
	state->format_ready = 1;
}

static void on_timeout(void *userdata, uint64_t expirations)
{
	struct pw_capture_state *state = userdata;
	(void)expirations;
	if (state->result->data == NULL) {
		state->result->error = strdup(
			"timed out waiting for a video frame from PipeWire");
	}
	pw_main_loop_quit(state->loop);
}

static void on_state_changed(void *userdata, enum pw_stream_state old,
			     enum pw_stream_state new_state,
			     const char *error)
{
	struct pw_capture_state *state = userdata;
	(void)old;

	if (new_state == PW_STREAM_STATE_ERROR) {
		if (error)
			state->result->error = strdup(error);
		else
			state->result->error = strdup("PipeWire stream error");
		pw_main_loop_quit(state->loop);
	}
}

static const struct pw_stream_events stream_events = {
	PW_VERSION_STREAM_EVENTS,
	.state_changed = on_state_changed,
	.param_changed = on_param_changed,
	.process = on_process,
};

// pw_capture_frame captures a single video frame from the first available
// PipeWire Video/Source node. The caller must free result->data and
// result->error.
static struct capture_result pw_capture_frame(int timeout_sec)
{
	struct capture_result result = {0};
	struct pw_capture_state state = {0};
	state.result = &result;

	pw_init(NULL, NULL);

	state.loop = pw_main_loop_new(NULL);
	if (state.loop == NULL) {
		result.error = strdup("could not create PipeWire main loop");
		return result;
	}

	// Query GPU-supported modifiers for BGRx (XRGB8888).
	struct drm_modifier_list modifiers = query_drm_modifiers();

	struct pw_properties *props = pw_properties_new(
		PW_KEY_MEDIA_TYPE,     "Video",
		PW_KEY_MEDIA_CATEGORY, "Capture",
		PW_KEY_MEDIA_ROLE,     "Screen",
		PW_KEY_TARGET_OBJECT,  "gamescope",
		NULL);

	state.stream = pw_stream_new_simple(
		pw_main_loop_get_loop(state.loop),
		"screenscope",
		props,
		&stream_events,
		&state);

	if (state.stream == NULL) {
		result.error = strdup("could not create PipeWire stream");
		pw_main_loop_destroy(state.loop);
		return result;
	}

	// Build format parameters. We offer two pods:
	//
	// Pod 0: BGRx with DRM modifier (DMA-BUF path). The modifier enum
	//        includes all modifiers the GPU supports, queried from DRM.
	//        Gamescope marks the modifier as MANDATORY, so this pod
	//        is required to match gamescope's format variant.
	//
	// Pod 1: BGRx/BGRA/RGBx/RGBA without modifier (MemFd fallback).
	//        Works with non-gamescope Video/Source nodes.

	uint8_t buffer[8192];
	struct spa_pod_builder b = SPA_POD_BUILDER_INIT(buffer, sizeof(buffer));
	const struct spa_pod *params[2];

	// Pod 0: DMA-BUF with modifier.
	struct spa_pod_frame f[2];
	spa_pod_builder_push_object(&b, &f[0],
		SPA_TYPE_OBJECT_Format, SPA_PARAM_EnumFormat);
	spa_pod_builder_add(&b,
		SPA_FORMAT_mediaType,    SPA_POD_Id(SPA_MEDIA_TYPE_video),
		SPA_FORMAT_mediaSubtype, SPA_POD_Id(SPA_MEDIA_SUBTYPE_raw),
		SPA_FORMAT_VIDEO_format, SPA_POD_Id(SPA_VIDEO_FORMAT_BGRx),
		0);
	spa_pod_builder_prop(&b, SPA_FORMAT_VIDEO_modifier,
		SPA_POD_PROP_FLAG_MANDATORY | SPA_POD_PROP_FLAG_DONT_FIXATE);
	spa_pod_builder_push_choice(&b, &f[1], SPA_CHOICE_Enum, 0);
	// First value is the default (preferred). Use LINEAR.
	spa_pod_builder_long(&b, (int64_t)DRM_FORMAT_MOD_LINEAR);
	// Remaining values are all modifiers the GPU supports.
	for (uint32_t i = 0; i < modifiers.count; i++)
		spa_pod_builder_long(&b, modifiers.mods[i]);
	spa_pod_builder_pop(&b, &f[1]);
	spa_pod_builder_add(&b,
		SPA_FORMAT_VIDEO_size, SPA_POD_CHOICE_RANGE_Rectangle(
			&SPA_RECTANGLE(1920, 1080),
			&SPA_RECTANGLE(1, 1),
			&SPA_RECTANGLE(8192, 8192)),
		SPA_FORMAT_VIDEO_framerate, SPA_POD_CHOICE_RANGE_Fraction(
			&SPA_FRACTION(0, 1),
			&SPA_FRACTION(0, 1),
			&SPA_FRACTION(1000, 1)),
		0);
	params[0] = spa_pod_builder_pop(&b, &f[0]);

	// Pod 1: MemFd fallback (no modifier).
	params[1] = spa_pod_builder_add_object(&b,
		SPA_TYPE_OBJECT_Format, SPA_PARAM_EnumFormat,
		SPA_FORMAT_mediaType,    SPA_POD_Id(SPA_MEDIA_TYPE_video),
		SPA_FORMAT_mediaSubtype, SPA_POD_Id(SPA_MEDIA_SUBTYPE_raw),
		SPA_FORMAT_VIDEO_format, SPA_POD_CHOICE_ENUM_Id(5,
			SPA_VIDEO_FORMAT_BGRx,
			SPA_VIDEO_FORMAT_BGRx,
			SPA_VIDEO_FORMAT_BGRA,
			SPA_VIDEO_FORMAT_RGBx,
			SPA_VIDEO_FORMAT_RGBA),
		SPA_FORMAT_VIDEO_size,   SPA_POD_CHOICE_RANGE_Rectangle(
			&SPA_RECTANGLE(1920, 1080),
			&SPA_RECTANGLE(1, 1),
			&SPA_RECTANGLE(8192, 8192)),
		SPA_FORMAT_VIDEO_framerate, SPA_POD_CHOICE_RANGE_Fraction(
			&SPA_FRACTION(0, 1),
			&SPA_FRACTION(0, 1),
			&SPA_FRACTION(1000, 1)));

	int res = pw_stream_connect(state.stream,
		PW_DIRECTION_INPUT,
		PW_ID_ANY,
		PW_STREAM_FLAG_AUTOCONNECT | PW_STREAM_FLAG_MAP_BUFFERS,
		params, 2);

	if (res < 0) {
		result.error = strdup("could not connect PipeWire stream");
		pw_stream_destroy(state.stream);
		pw_main_loop_destroy(state.loop);
		return result;
	}

	// Set a timeout so we don't hang forever if no source is available.
	struct spa_source *timer = pw_loop_add_timer(
		pw_main_loop_get_loop(state.loop), on_timeout, &state);
	if (timer != NULL) {
		struct timespec ts;
		ts.tv_sec = timeout_sec;
		ts.tv_nsec = 0;
		pw_loop_update_timer(
			pw_main_loop_get_loop(state.loop),
			timer, &ts, NULL, false);
	}

	pw_main_loop_run(state.loop);

	if (timer != NULL) {
		pw_loop_destroy_source(
			pw_main_loop_get_loop(state.loop), timer);
	}

	pw_stream_destroy(state.stream);
	pw_main_loop_destroy(state.loop);
	pw_deinit();

	return result;
}
*/
import "C" //nolint:gocritic // cgo requires standalone import

import (
	"fmt"
	"image"
	"unsafe" //nolint:gocritic // used with cgo, requires separate import block
)

const (
	spaVideoFormatRGBx = 7  // SPA_VIDEO_FORMAT_RGBx
	spaVideoFormatBGRx = 8  // SPA_VIDEO_FORMAT_BGRx
	spaVideoFormatRGBA = 11 // SPA_VIDEO_FORMAT_RGBA
	spaVideoFormatBGRA = 12 // SPA_VIDEO_FORMAT_BGRA

	pipewireTimeoutSec = 5
)

// ViaPipeWire captures a single frame from the first PipeWire
// Video/Source node (typically gamescope's composited output).
func ViaPipeWire() (*image.RGBA, error) {
	result := C.pw_capture_frame(C.int(pipewireTimeoutSec))

	if result.error != nil {
		msg := C.GoString(result.error)
		C.free(unsafe.Pointer(result.error))
		return nil, fmt.Errorf("pipewire: %s", msg)
	}

	if result.data == nil {
		return nil, fmt.Errorf("pipewire: no frame data received")
	}
	defer C.free(result.data)

	width := int(result.width)
	height := int(result.height)
	rowBytes := width * 4
	size := rowBytes * height

	raw := unsafe.Slice((*byte)(result.data), size)

	switch uint32(result.format) {
	case spaVideoFormatBGRx, spaVideoFormatBGRA:
		return ConvertBGRAToRGBA(raw, width, height)
	case spaVideoFormatRGBx:
		return convertRGBxToRGBA(raw, width, height)
	case spaVideoFormatRGBA:
		return convertRGBADirect(raw, width, height)
	default:
		return nil, fmt.Errorf("pipewire: unsupported video format %d", result.format)
	}
}

func convertRGBxToRGBA(data []byte, width, height int) (*image.RGBA, error) {
	expected := width * height * 4
	if len(data) != expected {
		return nil, fmt.Errorf("pixel data length %d does not match %dx%dx4=%d", len(data), width, height, expected)
	}

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	copy(img.Pix, data)

	// Force alpha to opaque (the 'x' byte may be zero).
	for i := 3; i < len(img.Pix); i += 4 {
		img.Pix[i] = 0xFF
	}

	return img, nil
}

func convertRGBADirect(data []byte, width, height int) (*image.RGBA, error) {
	expected := width * height * 4
	if len(data) != expected {
		return nil, fmt.Errorf("pixel data length %d does not match %dx%dx4=%d", len(data), width, height, expected)
	}

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	copy(img.Pix, data)
	return img, nil
}
