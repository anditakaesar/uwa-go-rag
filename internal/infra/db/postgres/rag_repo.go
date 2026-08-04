package postgres

type RagRepository struct {
	db DBExecutor
}

func NewRagRepository(db DBExecutor) *RagRepository {
	return &RagRepository{
		db: db,
	}
}
