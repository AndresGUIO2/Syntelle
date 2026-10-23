package nativemedia

import "errors"

// h264Color describes the input actually supplied to OpenH264: studio-range
// BT.601 matrix with the existing BT.709 display primaries/transfer. Without
// explicit VUI, a browser guesses BT.709 matrix for this HD portrait size.
var h264Color = [3]uint32{1, 1, 6}

func colorH264AccessUnit(data []byte) ([]byte, error) {
	var result []byte
	appended := 0
	_, begin := annexStart(data, 0)
	for begin >= 0 && begin < len(data) {
		next, nextBegin := annexStart(data, begin)
		end := next
		if end < 0 {
			end = len(data)
		}
		if data[begin]&31 == 7 {
			patched, err := colorSPS(data[begin:end])
			if err != nil {
				return nil, err
			}
			result = append(result, data[appended:begin]...)
			result = append(result, patched...)
			appended = end
		}
		begin = nextBegin
	}
	if result == nil {
		return data, nil
	}
	return append(result, data[appended:]...), nil
}

func annexStart(data []byte, from int) (int, int) {
	for i := from; i+2 < len(data); i++ {
		if data[i] != 0 || data[i+1] != 0 {
			continue
		}
		if data[i+2] == 1 {
			return i, i + 3
		}
		if i+3 < len(data) && data[i+2] == 0 && data[i+3] == 1 {
			return i, i + 4
		}
	}
	return -1, -1
}

type h264Bits struct {
	data []byte
	at   int
	err  error
}

func (r *h264Bits) read(n int) uint32 {
	if n < 0 || n > 32 || r.at+n > len(r.data)*8 {
		r.err = errors.New("truncated H264 color syntax")
		return 0
	}
	var value uint32
	for range n {
		value = value<<1 | uint32((r.data[r.at/8]>>uint(7-r.at%8))&1)
		r.at++
	}
	return value
}
func (r *h264Bits) ue() uint32 {
	zeros := 0
	for r.read(1) == 0 {
		zeros++
		if zeros > 31 || r.err != nil {
			r.err = errors.New("invalid bounded H264 exponential code")
			return 0
		}
	}
	return (uint32(1) << uint(zeros)) - 1 + r.read(zeros)
}

type h264Writer struct {
	data []byte
	n    int
}

func (w *h264Writer) write(value uint32, n int) {
	for i := n - 1; i >= 0; i-- {
		if w.n%8 == 0 {
			w.data = append(w.data, 0)
		}
		w.data[w.n/8] |= byte((value>>uint(i))&1) << uint(7-w.n%8)
		w.n++
	}
}
func (w *h264Writer) copy(data []byte, begin, end int) {
	for i := begin; i < end; i++ {
		w.write(uint32((data[i/8]>>uint(7-i%8))&1), 1)
	}
}

func spsVUI(r *h264Bits) (int, bool, error) {
	profile := r.read(8)
	if profile != 66 {
		return 0, false, errors.New("unexpected native H264 profile for color metadata")
	}
	r.read(8)
	r.read(8)
	r.ue() // constraints, level, SPS id
	r.ue() // log2_max_frame_num_minus4
	poc := r.ue()
	switch poc {
	case 0:
		r.ue()
	case 1:
		r.read(1)
		r.ue()
		r.ue()
		count := r.ue()
		if count > 256 {
			return 0, false, errors.New("unbounded H264 POC cycle")
		}
		for range count {
			r.ue()
		}
	case 2:
	default:
		return 0, false, errors.New("invalid H264 picture order count")
	}
	r.ue()
	r.read(1)
	r.ue()
	r.ue() // references, gaps, width, height
	if r.read(1) == 0 {
		r.read(1)
	}
	r.read(1) // direct_8x8_inference_flag
	if r.read(1) != 0 {
		for range 4 {
			r.ue()
		}
	}
	position := r.at
	present := r.read(1) != 0
	return position, present, r.err
}

func colorSPS(nal []byte) ([]byte, error) {
	if len(nal) < 5 || len(nal) > 4096 || nal[0]&31 != 7 {
		return nil, errors.New("invalid native H264 SPS")
	}
	rbsp := make([]byte, 0, len(nal)-1)
	for i := 1; i < len(nal); i++ {
		if i >= 3 && nal[i] == 3 && nal[i-1] == 0 && nal[i-2] == 0 {
			continue
		}
		rbsp = append(rbsp, nal[i])
	}
	trailing := len(rbsp) * 8
	for trailing > 0 && (rbsp[(trailing-1)/8]>>uint(7-(trailing-1)%8))&1 == 0 {
		trailing--
	}
	if trailing == 0 {
		return nil, errors.New("missing H264 RBSP trailing bit")
	}
	r := &h264Bits{data: rbsp}
	vui, present, err := spsVUI(r)
	if err != nil {
		return nil, err
	}
	w := &h264Writer{}
	if present {
		if r.read(1) != 0 {
			if r.read(8) == 255 {
				r.read(16)
				r.read(16)
			}
		}
		if r.read(1) != 0 {
			r.read(1)
		}
		signal := r.at
		if r.read(1) != 0 {
			r.read(3)
			r.read(1)
			if r.read(1) != 0 {
				r.read(8)
				r.read(8)
				r.read(8)
			}
		}
		if r.err != nil || r.at >= trailing {
			return nil, errors.New("truncated native H264 VUI")
		}
		w.copy(rbsp, 0, signal)
		writeColorSignal(w)
		// Preserve chroma location, timing, HRD, restrictions and trailing bit.
		w.copy(rbsp, r.at, trailing)
	} else {
		w.copy(rbsp, 0, vui)
		w.write(1, 1) // VUI present
		w.write(0, 1)
		w.write(0, 1) // no aspect/overscan override
		writeColorSignal(w)
		for range 6 {
			w.write(0, 1)
		} // chroma location, timing, NAL/VCL HRD, picture structure, restrictions
		w.write(1, 1) // rbsp_stop_one_bit
	}
	result := []byte{nal[0]}
	zeros := 0
	for _, b := range w.data {
		if zeros >= 2 && b <= 3 {
			result = append(result, 3)
			zeros = 0
		}
		result = append(result, b)
		if b == 0 {
			zeros++
		} else {
			zeros = 0
		}
	}
	return result, nil
}

func writeColorSignal(w *h264Writer) {
	w.write(1, 1)
	w.write(5, 3)
	w.write(0, 1)
	w.write(1, 1)
	for _, value := range h264Color {
		w.write(value, 8)
	}
}
