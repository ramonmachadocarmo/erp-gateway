package application

import "erp/services/gateway-service/internal/domain"

type Service struct {
	routes []domain.Route
}

func New(routes []domain.Route) *Service {
	return &Service{routes: routes}
}

func (s *Service) Resolve(path string) (domain.Route, string, error) {
	return domain.Match(s.routes, path)
}
