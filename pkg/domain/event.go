package domain

import (
	"time"
)

type Event struct {
	ID            string             `json:"id"`
	ArtistID      string             `json:"artist_id"`
	ArtistName    string             `json:"artist_name"`
	Title         string             `json:"title"`
	DateTime      time.Time          `json:"datetime"`
	Venue         Venue              `json:"venue"`
	TicketURL     string             `json:"ticket_url,omitempty"`
	TicketStatus  string             `json:"ticket_status,omitempty"`
	OnSaleDate    *time.Time         `json:"on_sale_date,omitempty"`
	ExternalIDs   EventExternalIDs   `json:"external_ids"`
	CreatedAt     time.Time          `json:"created_at"`
	UpdatedAt     time.Time          `json:"updated_at"`
	CachedUntil   time.Time          `json:"cached_until"`
}

type Venue struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	City      string  `json:"city"`
	Region    string  `json:"region,omitempty"`
	Country   string  `json:"country"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type EventExternalIDs struct {
	BandsintownID string `json:"bandsintown_id,omitempty"`
	TicketmasterID string `json:"ticketmaster_id,omitempty"`
}

type EventSearchRequest struct {
	ArtistName string  `json:"artist_name,omitempty"`
	ArtistID   string  `json:"artist_id,omitempty"`
	Location   string  `json:"location,omitempty"`
	Radius     int     `json:"radius,omitempty"`
	StartDate  *time.Time `json:"start_date,omitempty"`
	EndDate    *time.Time `json:"end_date,omitempty"`
}

type EventSearchResponse struct {
	Events []Event `json:"events"`
	Total  int     `json:"total"`
}