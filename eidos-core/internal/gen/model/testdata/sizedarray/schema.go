package schema

type Thing struct {
	Digest [32]byte `eidos:"both"`
}
