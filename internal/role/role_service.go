package role

import (
	"context"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
)

type IRoleRepository interface {
	FetchRoleByParam(ctx context.Context, param domain.FetchRoleParam) (*domain.Role, error)
	FetchAll(ctx context.Context, param *domain.FetchAllRoleParam) ([]domain.Role, error)
}

type RoleService struct {
	roleRepo IRoleRepository
}

type RoleServiceDep struct {
	RoleRepo IRoleRepository
}

func NewRoleService(dep RoleServiceDep) *RoleService {
	return &RoleService{
		roleRepo: dep.RoleRepo,
	}
}

func (s *RoleService) FetchAll(ctx context.Context, param domain.FetchAllRoleParam) ([]domain.Role, *domain.FetchAllRoleParam, error) {
	param.Normalize()
	users, err := s.roleRepo.FetchAll(ctx, &param)
	if err != nil {
		return nil, nil, err
	}
	return users, &param, nil
}

func (s *RoleService) Get(ctx context.Context, id int64) (*domain.Role, error) {
	role, err := s.roleRepo.FetchRoleByParam(ctx, domain.FetchRoleParam{
		ID: &id,
	})
	return role, err
}
