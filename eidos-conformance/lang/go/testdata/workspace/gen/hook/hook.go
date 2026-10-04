package hook

// Hook runs before a generator writes a file.
//
//+acme:stub
type Hook interface {
	Run(path string) error
}
