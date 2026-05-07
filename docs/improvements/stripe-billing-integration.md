# Stripe Billing Integration Plan (Freemium & Elements)

## Objective
Document the architecture and implementation plan for integrating Stripe (Freemium & Elements) into the Synthify application. This plan focuses on providing a "Free Tier" along with a custom-styled subscription flow using Stripe Elements.

## Architectural Design

### 1. Database Schema (`accounts` table)
Billing is managed at the `account` level. The existing `accounts` table will be extended:
- `stripe_customer_id`: TEXT (Stores the ID of the customer in Stripe)
- `stripe_subscription_id`: TEXT (Stores the current active subscription ID)
- `plan`: Existing column, values will be `free` or `pro`.
- Quota columns (`storage_quota_bytes`, etc.): Will be updated based on the active plan.

### 2. Backend Integration (Go)
The backend will act as the orchestrator between the frontend and Stripe.
- **Dependency**: `github.com/stripe/stripe-go/v78`
- **Intent Creation**: An endpoint to create a `SetupIntent` or `PaymentIntent`. This allows the frontend to securely collect card details via Elements without the backend seeing the sensitive data.
- **Webhook Listener**: A critical endpoint (`/api/webhooks/stripe`) to receive asynchronous updates from Stripe:
    - `checkout.session.completed`: Initial subscription success.
    - `invoice.payment_succeeded`: Recurring payment success (keep plan active).
    - `invoice.payment_failed`: Payment failure (notify user/limit access).
    - `customer.subscription.deleted`: Subscription cancelled (revert to `free` tier).

### 3. Frontend Integration (Next.js & Stripe Elements)
Custom UI for better branding and user experience.
- **Dependency**: `@stripe/stripe-js`, `@stripe/react-stripe-js`
- **Pricing Component**: Displays Free vs Pro features.
- **Payment Element**: A secure, pre-built UI component from Stripe embedded in the app's billing settings. It handles validation, localizations, and formatting automatically.

## Implementation Steps

### Phase 1: Infrastructure & Data
1. Add `STRIPE_SECRET_KEY` and `STRIPE_WEBHOOK_SECRET` to Secret Manager.
2. Add `stripe_customer_id` and `stripe_subscription_id` to the `accounts` table in `db/init/001_schema.sql`.
3. Update SQLC queries to include these new fields.

### Phase 2: Core Backend Logic
1. Implement a `BillingService` in Go to interact with the Stripe API.
2. Implement the Webhook handler to sync Stripe state with the local database.
3. Create an endpoint to fetch the current billing status and quotas for the frontend.

### Phase 3: Frontend Experience
1. Build the Billing/Settings page in `apps/web`.
2. Integrate the Stripe `Elements` provider and `PaymentElement`.
3. Handle the "Success" and "Error" states after payment confirmation.

## Enforcement of Free Tier Limits
The application will check the `accounts` table before performing high-cost operations:
- **Uploading Documents**: Compare `storage_used_bytes` vs `storage_quota_bytes`.
- **LLM Processing**: Check if the current plan allows the requested model or number of requests.

## Testing
- Use **Stripe CLI** to forward webhooks to the local backend during development.
- Use Stripe **Test Cards** (e.g., 4242...) to verify various payment scenarios.
