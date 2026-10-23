//go:build nativecodecs && cgo

package nativemedia

import (
	"errors"
	"image"
	"os"

	"github.com/pion/mediadevices/pkg/codec"
	"github.com/pion/mediadevices/pkg/codec/openh264"
	"github.com/pion/mediadevices/pkg/codec/opus"
	"github.com/pion/mediadevices/pkg/io/audio"
	"github.com/pion/mediadevices/pkg/io/video"
	"github.com/pion/mediadevices/pkg/prop"
	"github.com/pion/mediadevices/pkg/wave"
)

type nativeFactory struct{ videoMode string }

func defaultEncoders() (EncoderFactory, error) {
	mode, err := videoEncoderMode(os.Getenv("SYNTELLE_VIDEO_ENCODER"))
	if err != nil {
		return nil, err
	}
	return nativeFactory{videoMode: mode}, nil
}

type nativeVideo struct {
	frame  *image.YCbCr
	reader codec.ReadCloser
	key    codec.KeyFrameController
}

func (f nativeFactory) Video(width, height, fps, bitrate int) (VideoEncoder, error) {
	return selectVideoEncoder(f.videoMode,
		func() (VideoEncoder, error) { return newNVENCVideo(width, height, fps, bitrate) },
		func() (VideoEncoder, error) { return newOpenH264Video(width, height, fps, bitrate) })
}

func newOpenH264Video(width, height, fps, bitrate int) (VideoEncoder, error) {
	v := &nativeVideo{frame: image.NewYCbCr(image.Rect(0, 0, width, height), image.YCbCrSubsampleRatio420)}
	params, err := openh264.NewParams()
	if err != nil {
		return nil, err
	}
	params.BitRate = bitrate
	params.EnableFrameSkip = false
	params.IntraPeriod = uint(fps)
	params.MultipleThreadIdc = 1
	params.SliceMode = openh264.SMSingleSlice
	params.SliceNum = 1
	reader, err := params.BuildVideoEncoder(video.ReaderFunc(func() (image.Image, func(), error) { return v.frame, func() {}, nil }), prop.Media{Video: prop.Video{Width: width, Height: height, FrameRate: float32(fps)}})
	if err != nil {
		return nil, err
	}
	controller, ok := reader.(codec.Controllable)
	if !ok {
		closeErr := reader.Close()
		return nil, errors.Join(errors.New("native H264 has no encoder controller"), closeErr)
	}
	key, ok := controller.Controller().(codec.KeyFrameController)
	if !ok {
		closeErr := reader.Close()
		return nil, errors.Join(errors.New("native H264 cannot force keyframes"), closeErr)
	}
	v.reader = reader
	v.key = key
	return v, nil
}

func (v *nativeVideo) Encode(frame *image.RGBA) ([]byte, error) {
	if err := limitedI420(v.frame, frame); err != nil {
		return nil, err
	}
	data, release, err := v.reader.Read()
	if err != nil {
		return nil, err
	}
	// Pinned mediadevices v0.10.1 OpenH264.Read uses C.GoBytes: data is
	// owned by Go and remains valid after release, the next Read and Close.
	// Keep that ownership rather than copying the complete access unit again.
	release()
	return colorH264AccessUnit(data)
}
func (v *nativeVideo) ForceKeyFrame() error { return v.key.ForceKeyFrame() }
func (v *nativeVideo) Close() error         { return v.reader.Close() }

type nativeAudio struct {
	samples []int16
	reader  codec.ReadCloser
}

func (nativeFactory) Audio(rate, channels, bitrate int) (AudioEncoder, error) {
	a := &nativeAudio{}
	params, err := opus.NewParams()
	if err != nil {
		return nil, err
	}
	params.BitRate = bitrate
	params.Latency = opus.Latency20ms
	a.reader, err = params.BuildAudioEncoder(audio.ReaderFunc(func() (wave.Audio, func(), error) {
		return &wave.Int16Interleaved{Data: a.samples, Size: wave.ChunkInfo{Len: len(a.samples) / channels, Channels: channels, SamplingRate: rate}}, func() {}, nil
	}), prop.Media{Audio: prop.Audio{SampleRate: rate, ChannelCount: channels}})
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (a *nativeAudio) Encode(samples []int16) ([]byte, error) {
	a.samples = samples
	data, release, err := a.reader.Read()
	if err != nil {
		return nil, err
	}
	// Pinned mediadevices v0.10.1 Opus.Read allocates its output in Go;
	// its release callback is a no-op and later reads do not reuse this array.
	release()
	return data, nil
}
func (a *nativeAudio) Close() error { return a.reader.Close() }
