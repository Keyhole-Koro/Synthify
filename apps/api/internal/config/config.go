package config

import (
	"fmt"
	"os"

	"github.com/synthify/backend/internal/platform/storage"
)

type API struct {
	Port                     string
	Env                      string
	ReadinessKey             string
	CORSAllowedOrigins       string
	GCSBucket                string
	GCSUploadURLBase         string
	InternalGCSUploadBase    string
	FirebaseProjectID        string
	FirebaseAuthEmulatorHost string
	WorkerBaseURL            string
	Stripe                   Stripe
	Billing                  Billing
	NewRelic                 NewRelic
}

type Stripe struct {
	SecretKey        string
	WebhookSecret    string
	ProPriceID       string
	ProPriceIDJPY    string
	ProPriceIDUSD    string
	DefaultCurrency  string
	MeterInputEvent  string
	MeterOutputEvent string
	APIBase          string
	APIVersion       string
}

type Billing struct {
	SuccessURL      string
	CancelURL       string
	PortalReturnURL string
}

type NewRelic struct {
	AppName    string
	LicenseKey string
}

type Store struct {
	DatabaseDSN string
}

func LoadAPI() API {
	uploadBase := mustBaseURL("GCS_UPLOAD_URL_BASE", get("GCS_UPLOAD_URL_BASE", "http://127.0.0.1:4443"))
	return API{
		Port:                     get("PORT", "8080"),
		Env:                      get("ENV", "production"),
		ReadinessKey:             os.Getenv("SYNTHIFY_READINESS_KEY"),
		CORSAllowedOrigins:       get("CORS_ALLOWED_ORIGINS", "http://localhost:5173,http://127.0.0.1:5173,http://localhost:5174,http://127.0.0.1:5174,http://localhost:3000,http://127.0.0.1:3000"),
		GCSBucket:                get("GCS_BUCKET", "synthify-uploads"),
		GCSUploadURLBase:         uploadBase,
		InternalGCSUploadBase:    mustBaseURL("INTERNAL_GCS_UPLOAD_URL_BASE", get("INTERNAL_GCS_UPLOAD_URL_BASE", uploadBase)),
		FirebaseProjectID:        os.Getenv("FIREBASE_PROJECT_ID"),
		FirebaseAuthEmulatorHost: os.Getenv("FIREBASE_AUTH_EMULATOR_HOST"),
		WorkerBaseURL:            os.Getenv("WORKER_BASE_URL"),
		Stripe: Stripe{
			SecretKey:        os.Getenv("STRIPE_SECRET_KEY"),
			WebhookSecret:    os.Getenv("STRIPE_WEBHOOK_SECRET"),
			ProPriceID:       os.Getenv("STRIPE_PRO_PRICE_ID"),
			ProPriceIDJPY:    os.Getenv("STRIPE_PRO_PRICE_ID_JPY"),
			ProPriceIDUSD:    os.Getenv("STRIPE_PRO_PRICE_ID_USD"),
			DefaultCurrency:  get("STRIPE_DEFAULT_CURRENCY", "jpy"),
			MeterInputEvent:  os.Getenv("STRIPE_METER_INPUT_EVENT"),
			MeterOutputEvent: os.Getenv("STRIPE_METER_OUTPUT_EVENT"),
			APIBase:          get("STRIPE_API_BASE", "https://api.stripe.com"),
			APIVersion:       get("STRIPE_API_VERSION", "2025-06-30.basil"),
		},
		Billing: Billing{
			SuccessURL:      get("BILLING_SUCCESS_URL", "http://localhost:3000/workspaces?billing=success"),
			CancelURL:       get("BILLING_CANCEL_URL", "http://localhost:3000/workspaces?billing=cancel"),
			PortalReturnURL: get("BILLING_PORTAL_RETURN_URL", "http://localhost:3000/workspaces"),
		},
		NewRelic: NewRelic{
			AppName:    get("NEW_RELIC_APP_NAME", "synthify-api"),
			LicenseKey: os.Getenv("NEW_RELIC_LICENSE_KEY"),
		},
	}
}

func LoadStore() Store {
	return Store{DatabaseDSN: os.Getenv("DATABASE_DSN")}
}

func get(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func mustBaseURL(name, value string) string {
	if err := storage.ValidateBaseURL(value); err != nil {
		panic(fmt.Sprintf("%s is invalid: %v", name, err))
	}
	return value
}
