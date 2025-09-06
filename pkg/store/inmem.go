package store

type inmem struct {
}

func newInMemStore() (*inmem, error) {
	return &inmem{}, nil
}
