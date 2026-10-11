// ErrClosed reports a closed resource. TypeScript names no error values,
// so sentinel does not stamp it.
export let ErrClosed = new Error('closed');
