package bootstrap

import (
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"sarathi/app/config"
	"sarathi/internal/ai"
	"sarathi/internal/analytics"
	"sarathi/internal/booking"
	"sarathi/internal/cache"
	"sarathi/internal/common/constants"
	"sarathi/internal/daily_bus"
	"sarathi/internal/dispatch"
	"sarathi/internal/driver"
	"sarathi/internal/gps"
	"sarathi/internal/luxury_bus"
	"sarathi/internal/map_service"
	"sarathi/internal/notifications"
	"sarathi/internal/payments"
	"sarathi/internal/pickup_return"
	"sarathi/internal/premium"
	"sarathi/internal/pricing"
	"sarathi/internal/self_drive"
	"sarathi/internal/shared_ride"
	"sarathi/internal/stations/ev"
	"sarathi/internal/stations/petrol"
	"sarathi/internal/test_drive"
	"sarathi/internal/tourist"
	"sarathi/internal/transit_pass"
	"sarathi/internal/vehicles"
	"sarathi/internal/websocket"
)

// Services is the wired object graph shared by the api and worker binaries.
type Services struct {
	Cache       *cache.Service
	Locker      *cache.Locker
	RateLimiter *cache.RateLimiter

	ModelLoader *ai.ModelLoader
	Vehicles    *vehicles.Service
	Map         *map_service.Service
	Petrol      *petrol.Service
	EV          *ev.Service
	GPS         *gps.Service

	Driver    *driver.Service
	Dispatch  *dispatch.Service
	Booking   *booking.Service
	Shared    *shared_ride.Service
	Pickup    *pickup_return.Service
	Premium   *premium.Service
	PremCache *premium.Cache
	Tourist   *tourist.Service
	SelfDrive *self_drive.Service
	Transit   *transit_pass.Service
	DailyBus  *daily_bus.Service
	TestDrive *test_drive.Service
	Luxury    *luxury_bus.Service

	Payments  *payments.Service
	Notify    *notifications.Queue
	Hub       *websocket.Hub
	Pricing   *pricing.Service
	Analytics *analytics.Service

	routes []interface{ Register(*http.ServeMux) }
}

// BuildServices wires every module. Order matters only where one service
// registers hooks on another (payments listeners, booking hooks, dispatch filters).
func BuildServices(cfg *config.Config, db *pgxpool.Pool, rdb *redis.Client, log *slog.Logger) *Services {
	s := &Services{}

	// ---- Redis core
	s.Cache = cache.NewService(rdb)
	s.Locker = cache.NewLocker(rdb)
	s.RateLimiter = cache.NewRateLimiter(rdb, log)
	geo := cache.NewGeo(rdb)
	slots := cache.NewSlots(rdb)
	pub := websocket.NewPublisher(s.Cache)
	s.Hub = websocket.NewHub(rdb, cfg.InstanceID, log)

	// ---- platform
	s.Analytics = analytics.NewService(rdb, db, log)
	s.Notify = notifications.NewQueue(rdb, log)
	s.Notify.RegisterSender(notifications.ChannelPush, notifications.NewPushSender(rdb, cfg.Notify.FCMURL, cfg.Notify.FCMKey))
	s.Notify.RegisterSender(notifications.ChannelSMS, notifications.NewSMSSender(rdb, s.RateLimiter, cfg.Notify.SMSURL, cfg.Notify.SMSKey, cfg.Notify.SMSSender))
	s.Notify.RegisterSender(notifications.ChannelEmail, notifications.NewEmailSender(rdb, cfg.Notify.SMTPHost, cfg.Notify.SMTPPort, cfg.Notify.SMTPUser, cfg.Notify.SMTPPass, cfg.Notify.MailFrom))
	s.Payments = payments.NewService(rdb, db, payments.NewHMACGateway(cfg.Security.PaymentKeySecret, cfg.Security.WebhookSecret), s.Locker, log)
	s.Pricing = pricing.NewService(rdb, pricing.NewSurge(rdb, geo))

	// ---- Map part
	s.ModelLoader = ai.NewModelLoader(rdb, s.Locker, log)
	inference := ai.NewInference(ai.NewClient(cfg.AIEndpoint), s.ModelLoader, s.Cache)
	s.Vehicles = vehicles.NewService(vehicles.NewRepository(db), vehicles.NewCache(s.Cache),
		vehicles.NewHTTPProvider(cfg.Map.CarSpecProviderURL), s.Locker, log)
	mapRepo := map_service.NewRepository(db)
	s.Map = map_service.NewService(
		map_service.NewOSRMEngine(cfg.Map.RoutingURL),
		map_service.NewRouteCache(s.Cache),
		map_service.NewCarSpecCache(rdb, s.Vehicles),
		map_service.NewRoadAnalyzer(rdb, mapRepo, ai.NewRoadPredictor(inference), cfg.Map),
		map_service.NewCompatibility(rdb, cfg.Map),
		map_service.NewTraffic(rdb),
		ai.NewRoutePredictor(inference),
		mapRepo, cfg.Map.MaxAlternatives, log)

	petrolRepo := petrol.NewRepository(db)
	petrolCache := petrol.NewCache(rdb, geo)
	tracker := petrol.NewTrafficTracker(rdb)
	s.Petrol = petrol.NewService(petrolCache, tracker, petrol.NewCrowdRater(rdb, tracker, petrolCache, petrolRepo, log),
		petrolRepo, s.Vehicles, cfg.Map, log)
	s.EV = ev.NewService(ev.NewCache(rdb, geo), ev.NewRepository(db), s.Vehicles, cfg.Map, log)

	// ---- drivers, dispatch, bookings
	s.Driver = driver.NewService(driver.NewRepository(db), driver.NewStore(rdb), geo, rdb, log)
	s.Dispatch = dispatch.NewService(dispatch.NewFinder(rdb, geo), dispatch.NewStore(rdb), s.Locker, s.Driver, pub, s.Analytics, log)
	s.Booking = booking.NewService(booking.NewRepository(db), booking.NewCache(rdb), s.Cache, booking.NewReservations(rdb),
		s.Dispatch, s.Driver, s.Pricing, s.Payments, pub, s.Notify, s.Analytics, log)

	sharedStore := shared_ride.NewStore(rdb, geo)
	s.Shared = shared_ride.NewService(sharedStore, shared_ride.NewMatcher(sharedStore), s.Booking, s.Pricing, pub)
	s.Pickup = pickup_return.NewService(pickup_return.NewParking(rdb, geo, db), s.Booking, s.Pricing)
	s.PremCache = premium.NewCache(rdb, db)
	s.Premium = premium.NewService(s.PremCache, premium.NewAvailability(slots), s.Booking, s.Payments, log)
	s.Tourist = tourist.NewService(tourist.NewCache(rdb, slots), s.Booking, s.Payments, log)
	sdCache := self_drive.NewCache(s.Cache, geo, slots)
	s.SelfDrive = self_drive.NewService(sdCache, self_drive.NewCalendar(slots), self_drive.NewDeposit(sdCache, s.Payments),
		s.Booking, s.Payments, s.Vehicles, log)

	// ---- transit, buses, test drives
	transitStore := transit_pass.NewStore(rdb)
	qrGen := transit_pass.NewQRGenerator(cfg.Security.QRSigningKey, transitStore)
	s.Transit = transit_pass.NewService(transitStore, transit_pass.NewFares(rdb, db), qrGen,
		transit_pass.NewQRValidator(qrGen, transitStore, s.RateLimiter), s.Payments, s.Analytics, db, cfg.Security.QRTTL, log)
	s.DailyBus = daily_bus.NewService(daily_bus.NewStore(s.Cache, geo), daily_bus.NewSeats(slots), s.Payments, s.Notify, log)

	tdCache := test_drive.NewCache(rdb)
	tdFence := test_drive.NewGeofence(rdb, tdCache, pub)
	s.TestDrive = test_drive.NewService(tdCache, test_drive.NewEligibility(rdb, db), tdFence, test_drive.NewCommission(rdb), slots, s.Booking, s.Payments, log)
	s.Dispatch.SetFilter(constants.ClassTestDrive, tdFence)

	s.Luxury = luxury_bus.NewService(luxury_bus.NewStore(rdb), luxury_bus.NewSeats(slots), s.Payments, pub, s.Notify, log)

	// ---- GPS last: it feeds drivers, traffic, petrol crowd rating and test-drive zones
	s.GPS = gps.NewService(gps.NewRedisTracking(rdb), gps.NewGeofence(geo, s.Petrol, tdFence, log),
		gps.NewLocationRepository(db), s.Driver, s.Map, rdb, log)

	s.routes = []interface{ Register(*http.ServeMux) }{
		vehicles.NewHandler(s.Vehicles), map_service.NewHandler(s.Map), petrol.NewHandler(s.Petrol), ev.NewHandler(s.EV),
		gps.NewHandler(s.GPS), driver.NewHandler(s.Driver), dispatch.NewHandler(s.Dispatch), booking.NewHandler(s.Booking),
		shared_ride.NewHandler(s.Shared), pickup_return.NewHandler(s.Pickup), s.Premium, tourist.NewHandler(s.Tourist),
		self_drive.NewHandler(s.SelfDrive), transit_pass.NewHandler(s.Transit), daily_bus.NewHandler(s.DailyBus),
		test_drive.NewHandler(s.TestDrive), luxury_bus.NewHandler(s.Luxury), payments.NewHandler(s.Payments), s.Analytics,
	}
	return s
}

// Register mounts every module's routes.
func (s *Services) Register(mux *http.ServeMux) {
	for _, r := range s.routes {
		r.Register(mux)
	}
	mux.HandleFunc("GET /ws", s.Hub.ServeWS)
}
