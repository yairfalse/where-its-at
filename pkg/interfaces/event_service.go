package interfaces

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/yair/where-its-at/pkg/domain"
	"github.com/yair/where-its-at/pkg/integrations"
)

type EventService struct {
	repository       domain.EventRepository
	artistRepository domain.ArtistRepository
	bandsintownClient *integrations.BandsintownClient
}

func NewEventService(
	repository domain.EventRepository,
	artistRepository domain.ArtistRepository,
	bandsintownClient *integrations.BandsintownClient,
) *EventService {
	return &EventService{
		repository:        repository,
		artistRepository:  artistRepository,
		bandsintownClient: bandsintownClient,
	}
}

func (s *EventService) SearchArtistEvents(ctx context.Context, artistName string, location string, radius int) (*domain.EventSearchResponse, error) {
	if artistName == "" {
		return nil, domain.ErrInvalidRequest
	}

	artistName = strings.TrimSpace(artistName)

	now := time.Now()
	cachedEvents, err := s.repository.SearchByArtist(ctx, artistName, &now, nil)
	if err == nil && len(cachedEvents) > 0 {
		validEvents := s.filterValidCache(cachedEvents, now)
		if len(validEvents) > 0 {
			filtered := s.filterByLocation(validEvents, location, radius)
			return &domain.EventSearchResponse{
				Events: filtered,
				Total:  len(filtered),
			}, nil
		}
	}

	if s.bandsintownClient == nil {
		return &domain.EventSearchResponse{
			Events: []domain.Event{},
			Total:  0,
		}, nil
	}

	externalEvents, err := s.bandsintownClient.SearchEvents(ctx, artistName, location)
	if err != nil {
		if err == domain.ErrRateLimitExceeded {
			return nil, err
		}
		if len(cachedEvents) > 0 {
			return &domain.EventSearchResponse{
				Events: cachedEvents,
				Total:  len(cachedEvents),
			}, nil
		}
		return nil, domain.ErrExternalAPIFailure
	}

	if len(externalEvents) > 0 {
		if err := s.repository.DeleteExpiredCache(ctx); err != nil {
			// Log error but continue
		}

		if err := s.repository.CreateBatch(ctx, externalEvents); err != nil {
			// Log error but continue
		}
	}

	return &domain.EventSearchResponse{
		Events: externalEvents,
		Total:  len(externalEvents),
	}, nil
}

func (s *EventService) GetArtistEvents(ctx context.Context, artistID string) (*domain.EventSearchResponse, error) {
	if artistID == "" {
		return nil, domain.ErrInvalidRequest
	}

	now := time.Now()
	cachedEvents, err := s.repository.SearchByArtist(ctx, artistID, &now, nil)
	if err == nil && len(cachedEvents) > 0 {
		validEvents := s.filterValidCache(cachedEvents, now)
		if len(validEvents) > 0 {
			return &domain.EventSearchResponse{
				Events: validEvents,
				Total:  len(validEvents),
			}, nil
		}
	}

	artist, err := s.artistRepository.GetByID(ctx, artistID)
	if err != nil {
		return nil, err
	}

	if s.bandsintownClient == nil {
		return &domain.EventSearchResponse{
			Events: []domain.Event{},
			Total:  0,
		}, nil
	}

	externalEvents, err := s.bandsintownClient.GetArtistEvents(ctx, artist.Name)
	if err != nil {
		if err == domain.ErrRateLimitExceeded {
			return nil, err
		}
		if len(cachedEvents) > 0 {
			return &domain.EventSearchResponse{
				Events: cachedEvents,
				Total:  len(cachedEvents),
			}, nil
		}
		return nil, domain.ErrExternalAPIFailure
	}

	for i := range externalEvents {
		externalEvents[i].ArtistID = artistID
	}

	if len(externalEvents) > 0 {
		if err := s.repository.DeleteExpiredCache(ctx); err != nil {
			// Log error but continue
		}

		if err := s.repository.CreateBatch(ctx, externalEvents); err != nil {
			// Log error but continue
		}
	}

	return &domain.EventSearchResponse{
		Events: externalEvents,
		Total:  len(externalEvents),
	}, nil
}

func (s *EventService) GetEvent(ctx context.Context, id string) (*domain.Event, error) {
	if id == "" {
		return nil, domain.ErrInvalidRequest
	}

	event, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if event.CachedUntil.Before(time.Now()) {
		if strings.HasPrefix(event.ID, "bandsintown_") && s.bandsintownClient != nil {
			externalID := strings.TrimPrefix(event.ID, "bandsintown_")
			updatedEvents, err := s.bandsintownClient.GetArtistEvents(ctx, event.ArtistName)
			if err == nil {
				for _, e := range updatedEvents {
					if e.ExternalIDs.BandsintownID == externalID {
						e.ID = event.ID
						e.ArtistID = event.ArtistID
						if err := s.repository.Update(ctx, &e); err == nil {
							return &e, nil
						}
					}
				}
			}
		}
	}

	return event, nil
}

func (s *EventService) filterValidCache(events []domain.Event, now time.Time) []domain.Event {
	valid := make([]domain.Event, 0, len(events))
	for _, event := range events {
		if event.CachedUntil.After(now) {
			valid = append(valid, event)
		}
	}
	return valid
}

func (s *EventService) filterByLocation(events []domain.Event, location string, radius int) []domain.Event {
	if location == "" {
		return events
	}

	filtered := make([]domain.Event, 0, len(events))
	locationLower := strings.ToLower(location)

	for _, event := range events {
		venueLocation := strings.ToLower(fmt.Sprintf("%s, %s, %s", 
			event.Venue.City, event.Venue.Region, event.Venue.Country))
		
		if strings.Contains(venueLocation, locationLower) {
			filtered = append(filtered, event)
		}
	}

	return filtered
}