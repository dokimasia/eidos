package schema

type Symbol any

type Thing struct {
	Parts []*Part `eidos:"both,walk,owner"`
}

type Part struct {
	Host Symbol `eidos:"both,walk"`
}
