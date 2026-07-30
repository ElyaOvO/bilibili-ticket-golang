package cluster_service

import (
	"context"
	"fmt"

	"bilibili-ticket-golang/cluster/domain"
)

const (
	cloudFeatureTicket = "TICKET"
	cloudFeatureBWS    = "BWS"
)

type FeatureChecker interface {
	CheckFeature(feature, checkpoint string) error
}

// SetFeatureChecker wires one cached cloud-control gate into the employer,
// dispatcher, and all subsequently started in-process workers.
//
//wails:ignore
func (s *ClusterService) SetFeatureChecker(checker FeatureChecker) {
	s.featureChecker = checker
	s.dispatcher.SetSubmitAuthorizer(func(_ context.Context, taskType domain.TaskType) error {
		feature := cloudFeatureTicket
		if taskType == domain.TaskTypeBWS {
			feature = cloudFeatureBWS
		}
		return s.requireFeature(feature, "dispatch")
	})
	s.local.SetFeatureAuthorizer(func(feature, checkpoint string) error {
		return s.requireFeature(feature, checkpoint)
	})
}

func (s *ClusterService) requireFeature(feature, checkpoint string) error {
	if s.featureChecker == nil {
		return nil
	}
	if err := s.featureChecker.CheckFeature(feature, checkpoint); err != nil {
		return fmt.Errorf("cloud-control denied %s at %s: %w", feature, checkpoint, err)
	}
	return nil
}
