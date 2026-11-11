package sticker

import (
	"container/heap"
	"sort"
)

// escritorBits escribe bits de menos a más significativo (así lo pide VP8L).
type escritorBits struct {
	buf  []byte
	acum uint64
	n    uint
}

func (w *escritorBits) bits(v uint32, n uint) {
	if n == 0 {
		return
	}
	w.acum |= uint64(v&(1<<n-1)) << w.n
	w.n += n
	for w.n >= 8 {
		w.buf = append(w.buf, byte(w.acum))
		w.acum >>= 8
		w.n -= 8
	}
}

func (w *escritorBits) fin() []byte {
	if w.n > 0 {
		w.buf = append(w.buf, byte(w.acum))
		w.acum, w.n = 0, 0
	}
	return w.buf
}

// ---------- códigos de Huffman ----------

type nodoH struct {
	peso   int
	hijos  [2]int // -1 = hoja
	simbol int
}

type monton struct {
	nodos []nodoH
	ids   []int
}

func (m monton) Len() int { return len(m.ids) }
func (m monton) Less(i, j int) bool {
	a, b := m.nodos[m.ids[i]], m.nodos[m.ids[j]]
	if a.peso != b.peso {
		return a.peso < b.peso
	}
	return m.ids[i] < m.ids[j]
}
func (m monton) Swap(i, j int) { m.ids[i], m.ids[j] = m.ids[j], m.ids[i] }
func (m *monton) Push(x any)   { m.ids = append(m.ids, x.(int)) }
func (m *monton) Pop() any {
	n := len(m.ids)
	x := m.ids[n-1]
	m.ids = m.ids[:n-1]
	return x
}

// longitudesHuffman da el largo del código de cada símbolo (0 = no se usa), sin pasar de max. Con menos de dos símbolos usados devuelve
// un solo símbolo con largo 1 (quien llama usa entonces el código simple).
func longitudesHuffman(freq []int, max int) []int {
	for umbral := 1; ; umbral *= 2 {
		l, mayor := huffmanUna(freq, umbral)
		if mayor <= max {
			return l
		}
	}
}

func huffmanUna(freq []int, umbral int) ([]int, int) {
	out := make([]int, len(freq))
	m := &monton{}
	for s, f := range freq {
		if f > 0 {
			if f < umbral {
				f = umbral
			}
			m.nodos = append(m.nodos, nodoH{peso: f, hijos: [2]int{-1, -1}, simbol: s})
			m.ids = append(m.ids, len(m.nodos)-1)
		}
	}
	switch len(m.ids) {
	case 0:
		return out, 0
	case 1:
		out[m.nodos[0].simbol] = 1
		return out, 1
	}
	heap.Init(m)
	for m.Len() > 1 {
		a := heap.Pop(m).(int)
		b := heap.Pop(m).(int)
		m.nodos = append(m.nodos, nodoH{peso: m.nodos[a].peso + m.nodos[b].peso, hijos: [2]int{a, b}, simbol: -1})
		heap.Push(m, len(m.nodos)-1)
	}
	mayor := 0
	var baja func(id, prof int)
	baja = func(id, prof int) {
		n := m.nodos[id]
		if n.hijos[0] < 0 {
			out[n.simbol] = prof
			if prof > mayor {
				mayor = prof
			}
			return
		}
		baja(n.hijos[0], prof+1)
		baja(n.hijos[1], prof+1)
	}
	baja(m.ids[0], 0)
	return out, mayor
}

// codigosCanonicos asigna a cada símbolo su código (ya con los bits invertidos, listo para escribirse de menos a más significativo).
func codigosCanonicos(long []int) []uint32 {
	out := make([]uint32, len(long))
	idx := make([]int, 0, len(long))
	for s, l := range long {
		if l > 0 {
			idx = append(idx, s)
		}
	}
	sort.SliceStable(idx, func(i, j int) bool {
		if long[idx[i]] != long[idx[j]] {
			return long[idx[i]] < long[idx[j]]
		}
		return idx[i] < idx[j]
	})
	// asignación directa: por largo ascendente y símbolo ascendente
	var codigo uint32
	prev := 0
	for _, s := range idx {
		l := long[s]
		if prev != 0 {
			codigo++
		}
		codigo <<= uint(l - prev)
		prev = l
		out[s] = invierte(codigo, uint(l))
	}
	return out
}

func invierte(v uint32, n uint) uint32 {
	var r uint32
	for i := uint(0); i < n; i++ {
		r = r<<1 | v&1
		v >>= 1
	}
	return r
}
