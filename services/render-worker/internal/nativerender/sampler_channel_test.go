package nativerender

import "testing"

func TestSampleChannelMatchesExistingSampler(t *testing.T) {
	t.Parallel()
	texture := NewTexture(64, 32, false)
	for i := range texture.Pixels {
		texture.Pixels[i] = byte(i*73 + 19)
	}
	for _, test := range []struct {
		name string
		x, y float64
	}{
		{"first-center", 0, 0},
		{"negative-clamp", -2, -3},
		{"fractional", 1.3, 2.71},
		{"upper-edge", 63.99, 31.99},
		{"positive-clamp", 1000, 1000},
	} {
		t.Run(test.name, func(t *testing.T) {
			baseline := texture.Sample(test.x, test.y)
			for channel := range 4 {
				if got := texture.SampleChannel(test.x, test.y, channel); got != baseline[channel] {
					t.Fatalf("channel %d = %.18g; existing sampler = %.18g", channel, got, baseline[channel])
				}
			}
		})
	}
}

var sampleChannelBenchmarkResult float64

func BenchmarkSampleSingleChannel(b *testing.B) {
	texture := NewTexture(64, 32, false)
	for i := range texture.Pixels {
		texture.Pixels[i] = byte(i*73 + 19)
	}
	for _, single := range []bool{false, true} {
		name := "four-channel-reference"
		if single {
			name = "single-channel"
		}
		b.Run(name, func(b *testing.B) {
			var sum float64
			for range b.N {
				if single {
					sum += texture.SampleChannel(1.3, 2.71, 2)
				} else {
					sum += texture.Sample(1.3, 2.71)[2]
				}
			}
			sampleChannelBenchmarkResult = sum
		})
	}
}
