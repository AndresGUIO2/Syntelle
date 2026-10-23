// Preserve the scalar renderer's double-precision interpolation order.
export void syntelle_compose(uniform const uint32 pixels[], uniform const uint32 background[],
    uniform uint32 output[], uniform int32 stride,
    uniform const int32 xa[], uniform const int32 xb[], uniform const double xf[],
    uniform const int32 ya[], uniform const int32 yb[], uniform const double yf[],
    uniform int32 firstRow, uniform int32 endRow) {
  foreach (y = firstRow ... endRow, x = 0 ... 720) {
    int32 bg = y * 720 + x;
    int32 out = y * stride + x;
    if (xa[x] < 0 || ya[y] < 0) {
      output[out] = background[bg];
    } else {
      uint32 a = pixels[(ya[y]+xa[x])/4], b = pixels[(ya[y]+xb[x])/4];
      uint32 c = pixels[(yb[y]+xa[x])/4], d = pixels[(yb[y]+xb[x])/4];
      if (((a | b | c | d) >> 24) == 0) {
        output[out] = background[bg];
      } else {
        double remaining = 0;
        if (((a & b & c & d) >> 24) != 255) {
          double top = (double)(a>>24) + ((double)(b>>24)-(double)(a>>24))*xf[x];
          double bottom = (double)(c>>24) + ((double)(d>>24)-(double)(c>>24))*xf[x];
          remaining = (double)1 - (top + (bottom-top)*yf[y])/(double)255;
        }
        uint32 result = 0xff000000;
        for (uniform int k = 0; k < 3; ++k) {
          uniform int shift = k*8;
          double av=(double)((a>>shift)&255), bv=(double)((b>>shift)&255);
          double cv=(double)((c>>shift)&255), dv=(double)((d>>shift)&255);
          double top = av + (bv-av)*xf[x];
          double bottom = cv + (dv-cv)*xf[x];
          double value = top + (bottom-top)*yf[y] + (double)((background[bg]>>shift)&255)*remaining;
          result |= (uint32)min((double)255, value+(double)0.5) << shift;
        }
        output[out] = result;
      }
    }
  }
}
