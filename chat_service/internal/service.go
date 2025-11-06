package chat

type Service {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}

}