package store

type Handler struct {
	inmem *inmem
}

func New() (*Handler, error) {
	i, err := newInMemStore()
	if err != nil {
		return nil, err
	}

	return &Handler{
		inmem: i,
	}, nil
}

func (p *Handler) Ping() (bool, error) {
	return true, nil
}
