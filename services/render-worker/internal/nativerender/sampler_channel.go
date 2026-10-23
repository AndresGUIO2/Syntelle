package nativerender

// SampleChannel samples one normalized RGBA channel (0 through 3) using the
// exact clamp-to-edge/bilinear convention of Sample. It avoids calculating
// unused channels for displacement fields with only one active component.
func (texture *Texture) SampleChannel(x, y float64, channel int) float64 {
	if x < 0 {
		x = 0
	} else if x > float64(texture.Width-1) {
		x = float64(texture.Width - 1)
	}
	if y < 0 {
		y = 0
	} else if y > float64(texture.Height-1) {
		y = float64(texture.Height - 1)
	}
	x0, y0 := int(x), int(y)
	x1, y1 := min(x0+1, texture.Width-1), min(y0+1, texture.Height-1)
	fx, fy := x-float64(x0), y-float64(y0)
	a, b := (y0*texture.Width+x0)*4+channel, (y0*texture.Width+x1)*4+channel
	c, d := (y1*texture.Width+x0)*4+channel, (y1*texture.Width+x1)*4+channel
	top := float64(texture.Pixels[a]) + (float64(texture.Pixels[b])-float64(texture.Pixels[a]))*fx
	bottom := float64(texture.Pixels[c]) + (float64(texture.Pixels[d])-float64(texture.Pixels[c]))*fx
	return (top + (bottom-top)*fy) / 255
}
