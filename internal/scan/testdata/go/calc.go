package calc

// Add returns a + b.
func Add(a, b int) int {
	return a + b
}

type Acc struct {
	total int
}

func (a *Acc) Push(n int) {
	a.total = Add(a.total, n)
}

func Sum(ns ...int) int {
	acc := &Acc{}
	for _, n := range ns {
		acc.Push(n)
	}
	return acc.total
}
