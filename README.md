# ms-order

Order and subscription management service for paid packages and features.

The service manages packages, orders, invoices, and subscriptions backed by **MongoDB**. Master `subscription_types` are maintained in **PostgreSQL** to ensure relational integrity, and checkouts initiate payment transactions via internal HTTP calls to `ms-payment-gateway`.

---

## Table of Contents

- [Architecture Overview](#architecture-overview)
- [Public Endpoints](#public-endpoints)
  - [Package Catalog](#package-catalog)
  - [Checkout & Orders](#checkout--orders)
  - [Subscription Management](#subscription-management)
- [Internal Endpoints](#internal-endpoints)
  - [Internal Packages & Types](#internal-packages--types)
  - [Subscription Validation](#subscription-validation)
  - [Payment Notifications (Webhook)](#payment-notifications-webhook)
  - [Internal Lookups & Users](#internal-lookups--users)
- [Getting Started](#getting-started)
  - [Prerequisites](#prerequisites)
  - [Installation & Running](#installation--running)
  - [Running Tests](#running-tests)
- [Environment Configuration](#environment-configuration)

---

## Architecture Overview

- **Storage**:
  - **MongoDB**: Stores `packages`, `orders` (with embedded snapshot of invoice and payment status), and `subscriptions`.
  - **PostgreSQL**: Stores relational master data for `subscription_types` (auto-migrated via SQL files embedded in binary).
- **Payment Processing**:
  - Communicates with `ms-payment-gateway` over HTTP.
  - Supports standard gateway payment methods (e.g. Xendit) and internal **Fal Pay** (`gateway_code: "falpay"`), which debits user balance via `ms-e-wallet` and activates subscriptions immediately upon sufficient funds.
- **Identity & Authorization**:
  - In production, client requests pass through `ms-gateway` with the `/order` prefix.
  - The gateway extracts `customer_id` and `company_id` from the verified JWT.
  - `customer_id` represents the paying user (e.g. company admin), whereas the `subscription` is tied to `company_id` so that the subscription benefits apply organization-wide to all company members.
  - Client-supplied identity headers (`X-User-ID`, `X-Company-ID`) are stripped and securely injected by the gateway.

---

## Public Endpoints

Public routes are typically accessed via `ms-gateway` using the `/order` prefix (or directly during local testing):

### Package Catalog

- `GET /packages`: Retrieve list of available packages.
  - Default: Returns only active packages with the default `Subscription Package` master type (`8b7d8f9f-3b0a-4c89-a4d4-452ce5d763e1`).
  - Query parameters:
    - `subscription_type_id`: Filter by specific subscription type UUID.
    - `include_inactive=true`: Include inactive packages (for administration/back-office).
- `POST /packages`: Create a package.
- `PUT /packages/{id}`: Update an existing package by ID.
- `DELETE /packages/{id}`: Soft/hard delete a package by ID.
- `GET /subscription-types`: Retrieve list of all active subscription types.

### Checkout & Orders

- `POST /orders/checkout`: Initiate a new order checkout.
  - Client sends `package_id`, `duration_key`, and payment details.
  - Supported `duration_key` options:
    - `monthly`: 1 month (0% discount)
    - `quarter`: 3 months (2% discount)
    - `semiannual`: 6 months (5% discount)
    - `annual`: 12 months (20% discount)
  - Pricing, discounts, invoice creation, and payment gateway payloads are strictly calculated on the backend. Clients cannot supply arbitrary prices.

#### Checkout Request Example:

```bash
curl --location 'http://localhost:8080/order/orders/checkout' \
  --header 'Authorization: Bearer <admin_company_access_token>' \
  --header 'Content-Type: application/json' \
  --data '{
    "package_id": "pkg_monthly",
    "duration_key": "monthly",
    "gateway_code": "xendit",
    "customer_email": "admin@example.com"
  }'
```

#### Fal Pay Checkout:
To pay using Fal Pay wallet balance, supply `"gateway_code": "falpay"`. The service sends `customer_id` (from `X-User-ID`) to `ms-payment-gateway`, which coordinates with `ms-e-wallet` to debit the balance and activates the subscription in real-time.

- `GET /orders/{id}`: Retrieve order details for the authenticated customer.
- `POST /orders/{id}/payment/check`: Synchronously poll/re-check payment status from the payment gateway.
- `GET /orders/{id}/payment/events`: Server-Sent Events (SSE) endpoint providing real-time streaming updates of payment status transitions.
- `GET /orders/history`: Fetch order history for the current authenticated user.

### Subscription Management

- `GET /subscriptions/current`: Retrieve current active subscription details for the requesting company.
- `POST /subscriptions/activate`: Direct subscription activation without checkout (**Superadmin only**).
  - Used for manual provisioning, complimentary access, or customer support resolution.
  - Audited with `activation_source: "superadmin"` and `activated_by`.

#### Direct Activation Example:

```bash
curl --location 'http://localhost:8080/order/subscriptions/activate' \
  --header 'Authorization: Bearer <superadmin_access_token>' \
  --header 'Content-Type: application/json' \
  --data '{
    "company_id": "company_uuid",
    "package_id": "pkg_monthly",
    "duration_key": "annual"
  }'
```

---

## Internal Endpoints

All endpoints prefixed with `/internal/*` are strictly protected and only accept requests originating from trusted IP subnets configured in `INTERNAL_ALLOWED_CIDRS`.

### Internal Packages & Types

- `POST /internal/packages`: Create a package.
- `PUT /internal/packages/{id}`: Update a package.
- `DELETE /internal/packages/{id}`: Delete a package.
- `GET /internal/subscription-types`: List all subscription types.

#### Create Package Example:

```bash
curl --location 'http://localhost:8085/internal/packages' \
  --header 'Content-Type: application/json' \
  --data '{
    "id": "pkg_monthly",
    "subscription_type_id": "8b7d8f9f-3b0a-4c89-a4d4-452ce5d763e1",
    "name": "Monthly Package",
    "duration_count": 1,
    "duration_unit": "month",
    "price_amount": 150000,
    "currency": "IDR",
    "active": true
  }'
```

### Subscription Validation

- `POST /internal/subscription/validate`: Verifies whether a given company has an active, valid subscription period.

```bash
curl --location 'http://localhost:8085/internal/subscription/validate' \
  --header 'Content-Type: application/json' \
  --data '{
    "company_id": "company_uuid"
  }'
```

Response:
```json
{
  "allowed": true,
  "reason": "subscription is active"
}
```

### Payment Notifications (Webhook)

- `POST /internal/payments/notifications`: Webhook callback received from `ms-payment-gateway`.

```bash
curl --location 'http://localhost:8085/internal/payments/notifications' \
  --header 'Content-Type: application/json' \
  --data '{
    "order_id": "ord_123",
    "payment_id": "pay_123",
    "status": "paid",
    "external_id": "xendit-pay_123"
  }'
```

Status behavior:
- `paid`: Marks order as `paid` and creates/extends the company's active subscription period.
- `failed`: Updates order status to `payment_failed`.
- `expired`: Updates order status to `expired`.

### Internal Lookups & Users

- `GET /internal/orders/{id}`: Fetch any order by ID (bypassing customer ownership checks).
- `GET /internal/invoices/{id}`: Fetch invoice details by invoice ID.
- `POST /internal/users`: Create user internally (triggers sync hooks / Kafka event if enabled).
- `GET /internal/users`: List all users.
- `GET /internal/users/{id}`: Get user details by ID.

---

## Getting Started

### Prerequisites

- **Go**: version `1.22` or newer
- **MongoDB**: `v6.0+`
- **PostgreSQL**: `v14+`

### Installation & Running

1. **Clone the repository**:
   ```bash
   git clone <repo-url>
   cd ms-order
   ```

2. **Configure environment**:
   ```bash
   cp .env.example .env
   # Edit .env with your local database and service connection details
   ```

3. **Run the service**:
   ```bash
   go run ./cmd/api
   ```
   PostgreSQL migrations are executed automatically on service startup.

### Running Tests

Run all unit and integration tests:

```bash
go test -v ./...
```

---

## Environment Configuration

Configuration is loaded from `.env` using `github.com/joho/godotenv`. Refer to `.env.example` for the complete template.

| Variable | Type | Default / Example | Description |
| :--- | :--- | :--- | :--- |
| `APP_NAME` | string | `ms-order` | Application identifier |
| `APP_PORT` | string | `8085` | HTTP server listening port |
| `APP_SHUTDOWN_TIMEOUT` | duration | `10s` | Graceful shutdown timeout |
| `MONGO_URI` | string | `mongodb://localhost:27017` | MongoDB connection URI |
| `MONGO_DATABASE` | string | `ms_order` | MongoDB database name |
| `MONGO_TIMEOUT` | duration | `10s` | MongoDB client connection timeout |
| `DB_HOST` | string | `localhost` | PostgreSQL host |
| `DB_PORT` | int | `5432` | PostgreSQL port |
| `DB_USER` | string | `postgres` | PostgreSQL username |
| `DB_PASSWORD` | string | `postgres` | PostgreSQL password |
| `DB_NAME` | string | `fal_sync` | PostgreSQL database name |
| `DB_SSLMODE` | string | `disable` | PostgreSQL SSL mode (`disable`, `require`, etc.) |
| `DB_MAX_OPEN_CONNS` | int | `10` | Max open PostgreSQL connections |
| `DB_MAX_IDLE_CONNS` | int | `5` | Max idle PostgreSQL connections |
| `DB_CONN_MAX_LIFETIME` | duration | `30m` | Max connection lifetime |
| `DB_CONNECT_TIMEOUT` | duration | `5s` | PostgreSQL ping/connect timeout |
| `PAYMENT_GATEWAY_BASE_URL`| string | `http://localhost:8090` | Base URL for `ms-payment-gateway` |
| `PAYMENT_GATEWAY_TIMEOUT` | duration | `15s` | HTTP timeout for payment gateway calls |
| `PAYMENT_SUCCESS_URL_TEMPLATE` | string | `http://localhost:3000/payment/success?order_id={order_id}` | Redirect URL template for successful payments |
| `PAYMENT_FAILURE_URL_TEMPLATE` | string | `http://localhost:3000/payment/failed?order_id={order_id}` | Redirect URL template for failed payments |
| `PAYMENT_NOTIFICATION_URL` | string | `http://localhost:8085/internal/payments/notifications` | Webhook URL passed to payment gateway |
| `ORDER_INVOICE_DUE_DURATION` | duration | `24h` | Invoice expiration / due duration |
| `INTERNAL_ALLOWED_CIDRS` | csv | `127.0.0.0/8,::1/128,10.0.0.0/8,...` | Whitelisted CIDRs allowed to access `/internal/*` |
| `INTERNAL_TRUST_PROXY_HEADERS` | bool | `false` | Whether to trust `X-Forwarded-For` headers |
| `EXTERNAL_USER_SYNC_BASE_URL` | string | `""` | Optional HTTP user sync service URL |
| `EXTERNAL_USER_SYNC_PATH` | string | `/internal/users/sync` | User sync HTTP endpoint path |
| `EXTERNAL_USER_SYNC_TIMEOUT` | duration | `3s` | User sync request timeout |
| `KAFKA_ENABLED` | bool | `false` | Enable Kafka event publishing |
| `KAFKA_BROKERS` | csv | `localhost:9092` | Kafka broker list |
| `KAFKA_USER_CREATED_TOPIC` | string | `user.created` | Kafka topic for user creation events |
