// Package telemetry stores high-volume, append-only operational data in MongoDB:
// cab position history and trip event timelines. Writes are buffered and batched off
// the request path; if MongoDB is slow or down, records are dropped (and counted)
// rather than blocking trips. Nothing here is authoritative.
package telemetry

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	positionsTTL = 7 * 24 * time.Hour
	eventsTTL    = 90 * 24 * time.Hour
	bufferSize   = 4096
	batchSize    = 500
)

// Position is one cab location sample.
type Position struct {
	CabID   string    `bson:"cab_id" json:"-"`
	TripID  string    `bson:"trip_id,omitempty" json:"-"`
	Lat     float64   `bson:"lat" json:"lat"`
	Lng     float64   `bson:"lng" json:"lng"`
	Heading float64   `bson:"heading" json:"heading"`
	TS      time.Time `bson:"ts" json:"ts"`
}

// Event is one entry in a trip's timeline.
type Event struct {
	TripID string         `bson:"trip_id" json:"-"`
	Type   string         `bson:"type" json:"type"`
	Data   map[string]any `bson:"data,omitempty" json:"data,omitempty"`
	TS     time.Time      `bson:"ts" json:"ts"`
}

// Store is the telemetry sink. The zero value (and nil) is a valid no-op store.
type Store struct {
	client    *mongo.Client
	positions *mongo.Collection
	events    *mongo.Collection
	log       *slog.Logger
	queue     chan any
	wg        sync.WaitGroup
	dropped   atomic.Int64
}

// Open connects to MongoDB, ensures indexes and starts the background writer.
func Open(ctx context.Context, uri, database string, log *slog.Logger) (*Store, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(uri).
		SetServerSelectionTimeout(5 * time.Second).SetAppName("sarathi-api"))
	if err != nil {
		return nil, err
	}
	db := client.Database(database)
	s := &Store{
		client: client, positions: db.Collection("cab_positions"), events: db.Collection("trip_events"),
		log: log, queue: make(chan any, bufferSize),
	}
	if err := s.ensureIndexes(ctx); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, err
	}
	s.wg.Add(1)
	go s.writer()
	return s, nil
}

func (s *Store) ensureIndexes(ctx context.Context) error {
	if _, err := s.positions.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "trip_id", Value: 1}, {Key: "ts", Value: 1}}},
		{Keys: bson.D{{Key: "cab_id", Value: 1}, {Key: "ts", Value: -1}}},
		{Keys: bson.D{{Key: "ts", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(int32(positionsTTL.Seconds()))},
	}); err != nil {
		return err
	}
	_, err := s.events.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "trip_id", Value: 1}, {Key: "ts", Value: 1}}},
		{Keys: bson.D{{Key: "ts", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(int32(eventsTTL.Seconds()))},
	})
	return err
}

// Enabled reports whether a MongoDB backend is attached.
func (s *Store) Enabled() bool { return s != nil && s.client != nil }

func (s *Store) enqueue(v any) {
	if !s.Enabled() {
		return
	}
	select {
	case s.queue <- v:
	default:
		if s.dropped.Add(1)%1000 == 1 {
			s.log.Warn("telemetry_dropped", slog.Int64("total_dropped", s.dropped.Load()))
		}
	}
}

// RecordPosition queues a cab position sample.
func (s *Store) RecordPosition(p Position) { s.enqueue(p) }

// RecordEvent queues a trip timeline event.
func (s *Store) RecordEvent(tripID, typ string, data map[string]any) {
	s.enqueue(Event{TripID: tripID, Type: typ, Data: data, TS: time.Now().UTC()})
}

func (s *Store) writer() {
	defer s.wg.Done()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	var pos, evs []any
	flush := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if len(pos) > 0 {
			if _, err := s.positions.InsertMany(ctx, pos); err != nil {
				s.log.Warn("telemetry_write_failed", slog.String("collection", "cab_positions"), slog.String("error", err.Error()))
			}
			pos = pos[:0]
		}
		if len(evs) > 0 {
			if _, err := s.events.InsertMany(ctx, evs); err != nil {
				s.log.Warn("telemetry_write_failed", slog.String("collection", "trip_events"), slog.String("error", err.Error()))
			}
			evs = evs[:0]
		}
	}
	for {
		select {
		case v, ok := <-s.queue:
			if !ok {
				flush()
				return
			}
			switch v.(type) {
			case Position:
				pos = append(pos, v)
			case Event:
				evs = append(evs, v)
			}
			if len(pos)+len(evs) >= batchSize {
				flush()
			}
		case <-tick.C:
			flush()
		}
	}
}

// TripEvents returns a trip's timeline, oldest first.
func (s *Store) TripEvents(ctx context.Context, tripID string) ([]Event, error) {
	out := []Event{}
	if !s.Enabled() {
		return out, nil
	}
	cur, err := s.events.Find(ctx, bson.D{{Key: "trip_id", Value: tripID}},
		options.Find().SetSort(bson.D{{Key: "ts", Value: 1}}).SetLimit(200))
	if err != nil {
		return nil, err
	}
	return out, cur.All(ctx, &out)
}

// TripPath returns the recorded cab positions for a trip, oldest first.
func (s *Store) TripPath(ctx context.Context, tripID string) ([]Position, error) {
	out := []Position{}
	if !s.Enabled() {
		return out, nil
	}
	cur, err := s.positions.Find(ctx, bson.D{{Key: "trip_id", Value: tripID}},
		options.Find().SetSort(bson.D{{Key: "ts", Value: 1}}).SetLimit(5000))
	if err != nil {
		return nil, err
	}
	return out, cur.All(ctx, &out)
}

// Close flushes buffered records and disconnects.
func (s *Store) Close(ctx context.Context) {
	if !s.Enabled() {
		return
	}
	close(s.queue)
	s.wg.Wait()
	_ = s.client.Disconnect(ctx)
}

// Checker reports MongoDB readiness.
type Checker struct{ S *Store }

// Name implements httpapi.Checker.
func (Checker) Name() string { return "mongodb" }

// Check pings the primary.
func (c Checker) Check(ctx context.Context) error { return c.S.client.Ping(ctx, nil) }
