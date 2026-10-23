//go:build libyuv && cgo

#include <libyuv.h>
extern "C" int syntelle_rgba_i420(const uint8_t* source, int stride, uint8_t* y, int ys,
    uint8_t* u, int us, uint8_t* v, int vs, int width, int height) {
  return libyuv::ABGRToI420(source, stride, y, ys, u, us, v, vs, width, height);
}
