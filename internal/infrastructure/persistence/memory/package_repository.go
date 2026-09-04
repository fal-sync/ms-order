package memory

import (
	"context"
	"sync"

	"boilerplate-skeletoncode/internal/domain"
)

type PackageRepository struct {
	mu       sync.RWMutex
	packages map[string]domain.Package
}

func NewPackageRepository() *PackageRepository {
	return &PackageRepository{
		packages: make(map[string]domain.Package),
	}
}

func (r *PackageRepository) Create(_ context.Context, pkg domain.Package) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.packages[pkg.ID]; exists {
		return domain.ErrPackageAlreadyExists
	}

	r.packages[pkg.ID] = pkg
	return nil
}

func (r *PackageRepository) Update(_ context.Context, pkg domain.Package) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.packages[pkg.ID]; !exists {
		return domain.ErrPackageNotFound
	}

	r.packages[pkg.ID] = pkg
	return nil
}

func (r *PackageRepository) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.packages[id]; !exists {
		return domain.ErrPackageNotFound
	}

	delete(r.packages, id)
	return nil
}

func (r *PackageRepository) GetByID(_ context.Context, id string) (domain.Package, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	pkg, exists := r.packages[id]
	if !exists {
		return domain.Package{}, domain.ErrPackageNotFound
	}

	return pkg, nil
}

func (r *PackageRepository) List(_ context.Context, filter domain.PackageFilter) ([]domain.Package, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	packages := make([]domain.Package, 0, len(r.packages))
	for _, pkg := range r.packages {
		if filter.SubscriptionTypeID != "" && pkg.SubscriptionTypeID != filter.SubscriptionTypeID {
			continue
		}
		if !filter.IncludeInactive && !pkg.Active {
			continue
		}

		packages = append(packages, pkg)
	}

	return packages, nil
}

func (r *PackageRepository) ListActive(_ context.Context) ([]domain.Package, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	packages := make([]domain.Package, 0, len(r.packages))
	for _, pkg := range r.packages {
		if pkg.Active {
			packages = append(packages, pkg)
		}
	}

	return packages, nil
}
