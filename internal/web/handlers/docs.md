# ServiceConductor Documentation

## Overview

ServiceConductor is a self-hosted uptime monitoring tool. It regularly checks your websites and services, and sends you an alert when something goes down — so you find out before your users do.

- Each user's data is stored in its own separate database, so multiple users never slow each other down
- The dashboard is built into the server — no extra setup or build tools needed
- Everything ships as a single file you can run directly
- Supports Docker and Docker Compose for easy deployment
- Includes admin controls for managing users and access levels

---

## Quick Start

### Option 1 — Download the prebuilt binary

1. Go to the [Releases page](https://github.com/xdung24/conductor/releases) and download the latest binary for your operating system.
2. Run it:

```
./conductor
```

3. Open `http://localhost:3001` in your browser. A one-time sign-up link is printed in the terminal — use it to create your admin account at `/register?token=…`.

### Option 2 — Docker Compose

If you have Docker installed, this is the easiest way to get started:

```
docker compose up
```

The dashboard will be available at `http://localhost:3001`.

To expose the app through Cloudflare Tunnel, run:

```
CLOUDFLARE_TUNNEL_TOKEN=... docker compose --profile tunnel up
```

This starts an additional `cloudflared` service and does not require opening inbound ports publicly.

### Option 3 — Cloud / managed hosting

If you are using a hosted version of ServiceConductor, your administrator will give you the URL and either create an account for you or send you an invite link to sign up.

---

## Configuration

All settings are controlled through environment variables (values you set before starting the server):

| Variable | Default | Description |
|---|---|---|
| `LISTEN_ADDR` | `:3001` | The address and port the server listens on |
| `DATA_DIR` | `./data` | Where the server stores its database files |
| `SECRET_KEY` | `change-me-in-production` | A secret used to secure login sessions — **always change this in production** |

> **Important:** Never use the default `SECRET_KEY` on a real server. Set it to a long, random string.

---

## Monitor Types

Each monitor has a **type** that tells ServiceConductor how to check your service and which settings are available. You choose the type when creating or editing a monitor.

### HTTP / HTTPS

Loads a URL and checks that it responds correctly. Use this for any website or web API.

**Basic fields:**

- **URL** — the full web address to check, including `https://` (e.g. `https://example.com`)
- **Method** — the kind of request to send: `GET`, `POST`, `PUT`, `HEAD`, etc. (default: `GET`)
- **Expected Status** — the response code that means "working" (default: `200`, which is the normal success code)
- **Interval** — how often to check, in seconds (e.g. `60` means once per minute)
- **Timeout** — how many seconds to wait before giving up on a slow response

**Extra options:**

- **Keyword match** — check that a specific word or phrase appears (or doesn't appear) in the page content — useful for catching error pages that still return a "200 OK" status
- **JSON path query** — if the page returns structured data, check that a specific value in that data matches what you expect
- **Custom headers** — send extra information with each request (one `Key: Value` per line)
- **Request body** — text to send along with POST or PUT requests
- **Bearer token** — automatically adds a login token to each request as `Authorization: Bearer <token>`
- **Follow redirects** — whether to follow page redirects automatically (default: yes)
- **Accept invalid TLS** — skip certificate checks (only use this for internal/test servers)
- **TLS expiry alert** — get a warning N days before the site's security certificate expires
- **Max redirects** — limit how many redirects will be followed before stopping

### TCP

Checks that a server is accepting connections on a specific port. Useful for non-web services.

- **Host:Port** — the server address and port number (e.g. `db.example.com:5432`)
- **Timeout** — how long to wait for a connection before marking it as down

### Ping (ICMP)

Sends a ping to a host and checks that it responds. Use this to verify a server or device is reachable on the network.

- **Host** — hostname or IP address to ping

> On some Linux systems, pinging requires extra permissions. If pings fail unexpectedly, the server may need the `CAP_NET_RAW` capability, or be run as root.

### DNS

Checks that a domain name resolves to the correct address. Useful for catching DNS misconfigurations.

- **Hostname** — the domain to look up (e.g. `example.com`)
- **DNS Server** — the resolver to use, with port (e.g. `8.8.8.8:53`)
- **Record type** — the kind of record to check: `A`, `AAAA`, `CNAME`, `MX`, or `TXT`
- **Expected value** — the value you expect the lookup to return

### SMTP

Connects to an email server to verify it is running and accepting connections.

- **Host:Port** — the mail server address and port (e.g. `mail.example.com:587`)
- **Username / Password** — optional login credentials to test authenticated access

### Push / Heartbeat

A different kind of monitor — instead of ServiceConductor checking your service, *your service* sends a regular "still alive" signal to ServiceConductor. If the signal stops arriving within the expected interval, the monitor goes DOWN.

This is ideal for scheduled jobs, background scripts, or batch processes that run on a timer.

**How to set it up:**

1. Create a monitor with type **Push**.
2. Go to the monitor's detail page — you'll see a unique push URL (`GET /push/<token>`).
3. Configure your script or service to call that URL regularly, on an interval shorter than the monitor's configured interval.

> The push URL works without logging in. Keep your push token private — anyone with the URL can signal the monitor as UP.

### Database Monitors

Connects to a database server and checks that it is accessible. Available for the most common database systems:

| Type | Connection String Format |
|---|---|
| **MySQL / MariaDB** | `user:password@tcp(host:3306)/dbname` |
| **PostgreSQL** | `postgres://user:password@host:5432/dbname` |
| **Redis** | `redis://:password@host:6379/0` |
| **MongoDB** | `mongodb://user:password@host:27017/dbname` |

ServiceConductor attempts to connect (and ping) the database — UP means the connection succeeded, DOWN means it did not.

### Advanced Monitor Types

| Type | Description |
|---|---|
| **gRPC** | Checks a gRPC service using the standard health check protocol (`grpc.health.v1`) |
| **WebSocket** | Checks that a WebSocket server accepts connections |
| **MQTT** | Connects to an MQTT message broker and subscribes to a topic to verify it is reachable |
| **Kafka** | Connects to a Kafka broker and checks that it responds to metadata requests |
| **RabbitMQ** | Connects to a RabbitMQ message broker over AMQP |
| **SNMP** | Queries a network device using SNMP (retrieves a specific OID value) |
| **SIP** | Sends a SIP OPTIONS message to check that a VoIP/SIP server is responding |
| **Docker** | Checks that a named Docker container is in a running state |
| **System Service** | Checks the status of a system service (e.g. a systemd unit on Linux) |
| **Tailscale** | Checks that a peer device on your Tailscale network is reachable |
| **Globalping** | Uses the Globalping public network to run checks from multiple locations around the world |
| **Steam Game Server** | Queries a Steam/Source server over UDP using A2S_INFO |
| **GameDig** | Queries game servers using A2S or Quake 3 UDP protocol subsets |
| **Real Browser (Chromium)** | Opens the page in Chromium (local or remote DevTools endpoint) and checks rendered output |
| **Group** | Combines several monitors into one — the group is UP only when all members are UP |
| **Manual** | Status is set by hand — no automatic checks run |

---

## Notifications

Notification providers are the channels ServiceConductor uses to alert you. You set up a provider once and then attach it to as many monitors as you like. When a monitor changes state (goes down or comes back up), all its linked providers send an alert.

**Supported channels:** Slack, Discord, Telegram, Email (SMTP), ntfy, Webhook, Mattermost, Rocket.Chat, MS Teams, Google Chat, DingTalk, Feishu / Lark, WeCom, PagerDuty, Pushover, Gotify, Bark, Matrix, Signal, LINE Notify, SendGrid, Resend, Twilio SMS, Home Assistant, and more.

### Notification Logs

Every alert sent — whether it succeeded or failed — is recorded. Go to **Notifications → Logs** to review the history.

### Testing a Provider

After saving a notification provider, click the **Test** button on its row to send a test message immediately and confirm it's working.

---

## Status Pages

A status page is a public, read-only webpage showing the current status of your selected monitors. Share the link with customers or your team — they can see what's up or down without needing an account.

1. Go to **Status Pages → New**.
2. Give it a name and a short URL identifier (slug).
3. Select which monitors to show.
4. Save. The public URL is: `/status/<your-username>/<slug>`

No login is required to view a status page.

---

## Maintenance Windows

Maintenance windows let you silence alerts for selected monitors during a planned outage. Checks still run, but no notifications are sent while the window is active.

1. Go to your username menu → **Maintenance → New**.
2. Set the start and end date/time.
3. Select which monitors to cover.
4. Save.

---

## Tags

Tags are colour-coded labels you can attach to monitors for easy grouping and visual organisation on the dashboard. Manage them under your username menu → **Tags**.

---

## API Keys

API keys let external tools or scripts access ServiceConductor without using your password. They are sent in the `Authorization: Bearer <token>` header and work on all the same pages your browser session can access.

### Creating a Key

1. Username menu → **API Keys**.
2. Enter a descriptive name and click **Create**.
3. Copy the token immediately — it is only shown once.

### Using a Key

```
curl -H "Authorization: Bearer <your-token>" https://your-host/monitors/1/export
```

---

## Two-Factor Authentication (TOTP)

Two-factor authentication adds a second login step: after your password, you enter a short code from an app on your phone (Google Authenticator, Authy, or any TOTP-compatible app). This keeps your account safe even if your password is compromised.

1. Username menu → **2FA**.
2. Click **Set up 2FA** and scan the QR code with your authenticator app.
3. Enter the 6-digit code shown in the app to confirm.
4. From now on, every login will ask for a code after your password.

---

## Import / Export

Monitors can be saved to a JSON file and loaded back later — handy for backups, moving to a new server, or setting up the same monitor across multiple accounts.

### Export

On any monitor's detail page, click **Export** to download its configuration as a JSON file.

### Import

On the dashboard, click **Import** and upload a JSON file. Example files for every monitor type are available in the [examples/](https://github.com/xdung24/conductor/tree/main/examples) folder of the repository.

```json
{
  "name": "My API",
  "type": "http",
  "url": "https://api.example.com/healthz",
  "interval": 60,
  "timeout": 10
}
```

---

## User Management

Admins manage accounts from the **Admin → Users** page.

- **Create user** — add a new account with a chosen username and password.
- **Change password** — reset any user's password.
- **Toggle admin role** — promote a user to admin or remove admin rights.
- **Delete user** — permanently removes the account and all their data. You cannot delete your own account or the last remaining user.

---

## Registration

By default, nobody can sign up on their own — only admins can create accounts. You can change this under **Admin → Settings**:

- **Disabled** — only admins create accounts (the default).
- **Invite only** — generate a time-limited invite link from the Users page and share it as `/register?token=…`.
- **Open** — anyone who can reach the server can create an account.


ServiceConductor watches your websites and online services around the clock. If something goes down, it sends you an alert right away so you can act fast — instead of waiting for a customer to report the problem.

You can monitor websites, servers, email systems, databases, and more, all from one simple dashboard.

---

## Getting Started

### Creating Your Account

On your first visit, follow the link printed in the server console to create an admin account. Enter a username and password, and you are in.

If someone else already set up the server, ask them to invite you. You will receive a link like `/register?token=…` — open it in your browser and fill in your details.

---

## The Dashboard

When you log in, you land on the **Dashboard**. This is your home page. Each row is a monitor — a service you are watching.

- A **green "Up"** badge means the service is responding normally.
- A **red "Down"** badge means the service is not responding.
- A **grey "Unknown"** badge means the monitor has not run a check yet.

The small chart on each row shows recent response times so you can spot slowdowns at a glance.

---

## Monitors

A **monitor** is a check that runs automatically on a schedule. You tell it what to check, how often to check it, and what counts as "working."

### Creating a Monitor

1. Click **+ New Monitor** on the dashboard.
2. Give it a name (e.g. "Company Website").
3. Choose a **type** (see below).
4. Fill in the address or URL of the service.
5. Set how often to check it (the **interval**, in seconds — 60 means once per minute).
6. Click **Save**.

That's it. The monitor starts running automatically.

### Monitor Types

You don't need to understand the technical details — just pick the type that matches what you want to check:

| Type | Use it when… |
|---|---|
| **Website (HTTP/HTTPS)** | You want to check that a website loads correctly |
| **TCP Port** | You want to check that a server is accepting connections on a specific port |
| **Ping** | You want to check that a server or device is reachable on the network |
| **DNS** | You want to check that a domain name resolves to the correct address |
| **Email Server (SMTP)** | You want to check that an email server is running |
| **Heartbeat / Push** | Your own application sends a "still alive" signal — you get alerted if it stops |
| **MySQL / PostgreSQL / Redis / MongoDB** | You want to check that a database server is accessible |
| **gRPC / WebSocket / MQTT / Kafka / RabbitMQ** | Advanced service checks — ask your developer which applies |
| **Group** | Combines several monitors into one combined status |
| **Manual** | You control the status yourself, no automatic checks |

### Website Checks — Extra Options

For **Website (HTTP/HTTPS)** monitors, you can go beyond a simple "does it load?" check:

- **Keyword check** — make sure a specific word or phrase appears (or doesn't appear) on the page. Useful for detecting maintenance pages or error messages.
- **Custom headers / login token** — if the page requires authentication, you can provide credentials.
- **TLS certificate expiry** — get an alert before your SSL certificate expires, so your site never goes "Not Secure" unexpectedly.

### Heartbeat Monitors

A **Heartbeat** monitor works the other way around: instead of ServiceConductor reaching out to check your service, *your service* sends a regular "I'm alive" ping to ServiceConductor.

This is useful for scheduled jobs (e.g. a nightly backup script). If the job fails and stops sending its ping, you get an alert.

After creating a Heartbeat monitor, you'll see a unique URL on the monitor's detail page. Give that URL to your developer or paste it into your script.

### Pausing a Monitor

You can temporarily stop checks without deleting the monitor. Open the monitor and click **Pause**. Click **Resume** when you're ready to start monitoring again.

---

## Notifications

A **notification** is how ServiceConductor tells you when something goes wrong (or comes back up). You set up a notification provider once, then attach it to any monitors you like.

### Setting Up a Notification

1. Go to **Notifications** in the top menu.
2. Click **+ New Provider**.
3. Choose how you want to be notified (email, Slack, Telegram, etc.).
4. Fill in the required details and click **Save**.
5. Use the **Test** button to send a test message and confirm it's working.

### Linking Notifications to a Monitor

When creating or editing a monitor, scroll down to the **Notifications** section and tick the providers you want to alert for that monitor.

### Supported Notification Channels

ServiceConductor can send alerts through many channels, including:

- **Email**
- **Slack** and **Discord**
- **Telegram**
- **Microsoft Teams**
- **ntfy** (push notifications to your phone)
- **PagerDuty** (for on-call teams)
- **Webhook** (for custom integrations)
- …and many more

### Notification History

Every alert sent (and any that failed) is logged. Go to **Notifications → Logs** to review the history.

---

## Status Pages

A **Status Page** is a simple public webpage that shows the health of your selected services. You can share the link with customers or your team — they can see what's up or down without logging in.

### Creating a Status Page

1. Open the menu under your username and click **Status Pages**.
2. Click **+ New Status Page**.
3. Give it a name and a short URL slug (e.g. `my-company`).
4. Tick the monitors to include.
5. Click **Save**.

The public URL will be: `https://your-server/status/your-username/my-company`

---

## Maintenance Windows

A **Maintenance Window** tells ServiceConductor to pause alerts for selected monitors during a planned period — for example, while you perform a server upgrade. Checks still run, but no notifications are sent.

### Creating a Maintenance Window

1. Open the menu under your username and click **Maintenance**.
2. Click **+ New Window**.
3. Set the start and end date/time.
4. Tick the monitors to silence.
5. Click **Save**.

---

## Tags

**Tags** are coloured labels you can attach to monitors to keep things organised — for example, tagging all monitors belonging to a particular project or client.

Manage tags under your username menu → **Tags**.

---

## API Keys

An **API key** lets a program or script access ServiceConductor on your behalf, without needing your password.

You probably won't need this unless you have a developer integrating ServiceConductor with another tool.

To create one: username menu → **API Keys** → enter a name → **Create**. Copy the token shown — it will not be shown again.

---

## Two-Factor Authentication (2FA)

**2FA** adds a second step to your login — after your password, you enter a short code from an app on your phone. This keeps your account safe even if your password is stolen.

**To enable 2FA:**

1. Username menu → **2FA**.
2. Click **Set Up 2FA**.
3. Scan the QR code with an authenticator app (Google Authenticator, Authy, or similar).
4. Enter the 6-digit code shown in the app to confirm.

From then on, you will be asked for a code each time you log in.

---

## Your Account

### Changing Your Password

Ask an admin to reset your password via **Admin → Users**.

### Themes

Use the **sun/moon button** (☀) in the top bar to switch between dark and light mode.

---

## Admin: Managing Users

*This section is for administrators only.*

Go to **Admin → Users** to:

- **Add a new user** — create an account with a username and password.
- **Change a password** — reset someone's password.
- **Make someone an admin** — toggle the admin role on or off.
- **Delete a user** — permanently removes the account and all their monitors. This cannot be undone.

### Inviting Someone

Instead of creating an account for someone, you can send them an invite link:

1. Go to **Admin → Users**.
2. Click **Generate Invite**.
3. Share the link. It expires after a short time.

### Registration Settings

By default, nobody can sign up on their own. Under **Admin → Settings** you can choose:

- **Disabled** — only admins can create accounts (the default).
- **Invite only** — people can register only if they have a valid invite link.
- **Open** — anyone who can reach the server can create an account.

