# Where It's At

Two turntables and a microphone.

## What's Working

Pulls music and events from multiple sources:
- Spotify, Apple Music, YouTube Music, Deezer, SoundCloud
- Songkick, Ticketmaster, Eventbrite, Setlist.fm
- Resident Advisor and Bandcamp scrapers

## What's Not

- No frontend
- Not fully wired up yet

## What's Working (Backend)

- SQLite database with full CRUD operations
- Config management (supports JSON config + env vars)
- Independent module architecture (domain, collectors, integrations, interfaces, config)

## API

```
GET /api/search/artists?q=query
GET /api/search/events?artist=name  
GET /api/search/events/location?city=Berlin
GET /api/sources
```

## Run It (eventually)

```bash
go build ./cmd/where-its-at
./where-its-at
```

That was a good drum break.
