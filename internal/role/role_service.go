package role

import (
	"context"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
)

type RoleRepository interface {
	FetchRoleByParam(ctx context.Context, param domain.FetchRoleParam) (*domain.Role, error)
	FetchAll(ctx context.Context, param *domain.FetchAllRoleParam) ([]domain.Role, error)
}

type Service struct {
	roleRepo RoleRepository
}

type ServiceDependency struct {
	RoleRepo RoleRepository
}

func NewRoleService(dep ServiceDependency) *Service {
	return &Service{
		roleRepo: dep.RoleRepo,
	}
}

func (s *Service) FetchAll(ctx context.Context, param domain.FetchAllRoleParam) ([]domain.Role, *domain.FetchAllRoleParam, error) {
	param.Normalize()
	users, err := s.roleRepo.FetchAll(ctx, &param)
	if err != nil {
		return nil, nil, err
	}
	return users, &param, nil
}

func (s *Service) Get(ctx context.Context, id int64) (*domain.Role, error) {
	role, err := s.roleRepo.FetchRoleByParam(ctx, domain.FetchRoleParam{
		ID: &id,
	})
	return role, err
}
