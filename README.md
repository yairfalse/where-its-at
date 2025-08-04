# Where It's At

Two turntables and a microphone.

## What's Working

Pulls music and events from multiple sources:
- Spotify, Apple Music, YouTube Music, Deezer, SoundCloud
- Songkick, Ticketmaster, Eventbrite, Setlist.fm
- Resident Advisor and Bandcamp scrapers

## What's Not

- No config management yet
- No database 
- No frontend
- Not wired up to actually run

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
