package service

import (
	"context"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
)

type IRoleService interface {
	FetchAll(ctx context.Context, param domain.FetchAllRoleParam) ([]domain.Role, *domain.FetchAllRoleParam, error)
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
