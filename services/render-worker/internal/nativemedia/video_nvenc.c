//go:build nvenc && nativecodecs && cgo && linux && amd64

#include "video_nvenc.h"
#include <libavcodec/avcodec.h>
#include <libavutil/opt.h>
#include <libavutil/log.h>
#include <ffnvcodec/dynlink_cuda.h>
#include <libavutil/hwcontext.h>
#include <libavutil/hwcontext_cuda.h>
#include <pthread.h>

static pthread_once_t log_once = PTHREAD_ONCE_INIT;
static void quiet_log(void) { av_log_set_level(AV_LOG_QUIET); }

struct syntelle_nvenc {
    AVCodecContext *context;
    AVFrame *frame;
    AVFrame *device_frame;
    AVBufferRef *device;
    AVBufferRef *frames;
    AVPacket *packet;
    int64_t pts;
    int gpu;
};

void syntelle_nvenc_close(syntelle_nvenc *v) {
    if (!v) return;
    av_packet_free(&v->packet);
    av_frame_free(&v->frame);
    av_frame_free(&v->device_frame);
    avcodec_free_context(&v->context);
    av_buffer_unref(&v->frames);
    av_buffer_unref(&v->device);
    av_free(v);
}

syntelle_nvenc *syntelle_nvenc_open(int width, int height, int fps, int bitrate, int gpu, int *status) {
    // This private minimal FFmpeg build is used only here. Emit bounded Go
    // diagnostics instead of native driver output (no global application codec).
    pthread_once(&log_once, quiet_log);
    *status = AVERROR(ENOMEM);
    syntelle_nvenc *v = av_mallocz(sizeof(*v));
    if (!v) return NULL;
    v->gpu = gpu;
    const AVCodec *codec = avcodec_find_encoder_by_name("h264_nvenc");
    if (!codec) { *status = AVERROR_ENCODER_NOT_FOUND; goto fail; }
    v->context = avcodec_alloc_context3(codec);
    v->frame = av_frame_alloc();
    v->device_frame = av_frame_alloc();
    v->packet = av_packet_alloc();
    if (!v->context || !v->frame || !v->device_frame || !v->packet) goto fail;
    if (gpu) {
    *status = av_hwdevice_ctx_create(&v->device, AV_HWDEVICE_TYPE_CUDA, "0", NULL, AV_CUDA_USE_PRIMARY_CONTEXT);
    if (*status < 0) goto fail;
    v->frames = av_hwframe_ctx_alloc(v->device);
    if (!v->frames) goto fail;
    AVHWFramesContext *frames = (AVHWFramesContext*)v->frames->data;
    frames->format = AV_PIX_FMT_CUDA; frames->sw_format = AV_PIX_FMT_YUV420P;
    frames->width = width; frames->height = height; frames->initial_pool_size = 2;
    *status = av_hwframe_ctx_init(v->frames);
    if (*status < 0) goto fail;
    }
    AVCodecContext *c = v->context;
    c->width = width; c->height = height;
    c->pix_fmt = gpu ? AV_PIX_FMT_CUDA : AV_PIX_FMT_YUV420P;
    if (gpu) { c->hw_frames_ctx = av_buffer_ref(v->frames); if (!c->hw_frames_ctx) goto fail; }
    c->time_base = (AVRational){1, fps};
    c->framerate = (AVRational){fps, 1};
    c->bit_rate = bitrate; c->rc_max_rate = bitrate;
    // VBV is a rate-control budget, not a pending-frame queue. Keep enough
    // room for detailed face IDRs rather than forcing a two-frame quality dip.
    c->rc_buffer_size = bitrate;
    c->gop_size = fps; c->max_b_frames = 0;
    c->thread_count = 1;
    c->flags |= AV_CODEC_FLAG_LOW_DELAY;
    c->color_range = AVCOL_RANGE_MPEG;
    c->colorspace = AVCOL_SPC_SMPTE170M;
    c->color_primaries = AVCOL_PRI_BT709;
    c->color_trc = AVCOL_TRC_BT709;
    AVDictionary *options = NULL;
    av_dict_set(&options, "preset", "p5", 0);
    av_dict_set(&options, "tune", "ll", 0);
    av_dict_set(&options, "profile", "baseline", 0);
    av_dict_set(&options, "level", width == 1440 ? "4.0" : "3.1", 0);
    av_dict_set(&options, "rc", "vbr", 0);
    av_dict_set(&options, "rc-lookahead", "0", 0);
    av_dict_set(&options, "zerolatency", "1", 0);
    av_dict_set(&options, "delay", "0", 0);
    av_dict_set(&options, "surfaces", "2", 0);
    av_dict_set(&options, "forced-idr", "1", 0);
    // No GLOBAL_HEADER: pinned NVENC repeats SPS/PPS on every IDR.
    *status = avcodec_open2(c, codec, &options);
    // Unknown options are configuration failures, never silent changes.
    if (*status >= 0 && av_dict_count(options)) *status = AVERROR(EINVAL);
    av_dict_free(&options);
    if (*status < 0) goto fail;
    v->frame->format = AV_PIX_FMT_YUV420P;
    v->frame->width = width; v->frame->height = height;
    v->frame->color_range = c->color_range;
    v->frame->colorspace = c->colorspace;
    v->frame->color_primaries = c->color_primaries;
    v->frame->color_trc = c->color_trc;
    if (gpu) {
    *status = av_hwframe_get_buffer(v->frames, v->device_frame, 0);
    if (*status < 0) goto fail;
    }
    return v;
fail:
    syntelle_nvenc_close(v);
    return NULL;
}

int syntelle_nvenc_frame(syntelle_nvenc *v, uint8_t **y, uint8_t **u, uint8_t **cr, int *ys, int *cs) {
    int result = v->frame->buf[0] ? av_frame_make_writable(v->frame) : av_frame_get_buffer(v->frame,32);
    if (result < 0) return result;
    if (v->frame->linesize[1] != v->frame->linesize[2]) return AVERROR(EINVAL);
    *y = v->frame->data[0]; *u = v->frame->data[1]; *cr = v->frame->data[2];
    *ys = v->frame->linesize[0]; *cs = v->frame->linesize[1];
    return 0;
}

int syntelle_nvenc_surface(syntelle_nvenc *v, uint64_t *context, uint64_t *planes, int *strides) {
    if (!v->gpu) return AVERROR(EINVAL);
    int status = av_frame_make_writable(v->device_frame);
    if (status < 0) return status;
    AVHWDeviceContext *device = (AVHWDeviceContext*)v->device->data;
    AVCUDADeviceContext *cuda = device->hwctx;
    *context = (uintptr_t)cuda->cuda_ctx;
    for (int i=0; i<3; i++) { planes[i]=(uintptr_t)v->device_frame->data[i]; strides[i]=v->device_frame->linesize[i]; }
    return 0;
}

static int submit(syntelle_nvenc *v, AVFrame *frame, int key, uint8_t **data, int *size) {
    av_packet_unref(v->packet);
    frame->pts = v->pts++;
    frame->pict_type = key ? AV_PICTURE_TYPE_I : AV_PICTURE_TYPE_NONE;
    int result = avcodec_send_frame(v->context, frame);
    if (result < 0) return result;
    result = avcodec_receive_packet(v->context, v->packet);
    // Even EAGAIN is a failure: the worker requires one immediate access unit
    // per input, never a delayed queue surviving interruption or a new epoch.
    if (result < 0) return result;
    if (!v->packet->size || v->packet->pts != frame->pts) return AVERROR(EINVAL);
    *data = v->packet->data; *size = v->packet->size;
    return 0;
}

int syntelle_nvenc_encode_device(syntelle_nvenc *v,int key,uint8_t **data,int *size) {
    if (!v->gpu) return AVERROR(EINVAL);
    return submit(v,v->device_frame,key,data,size);
}

int syntelle_nvenc_encode(syntelle_nvenc *v, int key, uint8_t **data, int *size) {
    if (!v->gpu) return submit(v,v->frame,key,data,size);
    int result = av_frame_make_writable(v->device_frame);
    if (result < 0) return result;
    result = av_hwframe_transfer_data(v->device_frame, v->frame, 0);
    if (result < 0) return result;
    return syntelle_nvenc_encode_device(v, key, data, size);
}

// Explicit offline inspection only; never called by the media loop.
int syntelle_nvenc_download(syntelle_nvenc *v) {
    int status=v->frame->buf[0]?av_frame_make_writable(v->frame):av_frame_get_buffer(v->frame,32);
    if(status<0)return status;
    return av_hwframe_transfer_data(v->frame,v->device_frame,0);
}
