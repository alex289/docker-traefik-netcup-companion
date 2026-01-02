# Docker Traefik Netcup Companion

> Credits to
> - [netcup-dns-api](https://github.com/aellwein/netcup-dns-api)
> - [docker-traefik-cloudflare-companion](https://github.com/tiredofit/docker-traefik-cloudflare-companion)

A lightweight Go application that automatically creates DNS records in Netcup when Docker containers with Traefik labels are started.

## Features

- 🐳 Watches Docker container events in real-time
- 🏷️ Detects Traefik `Host` rules from container labels
- 🌐 Automatically creates/updates DNS A records in Netcup
- 🎯 Optional filtering by Docker labels
- 🧪 Dry run mode for testing without making actual DNS changes
- 🔔 Optional webhook notifications for DNS changes and errors (via [nicholas-fedor/shoutrrr](https://shoutrrr.nickfedor.com/))

## How It Works

1. The companion watches for Docker container start events
2. When a container starts, it inspects the container's labels for Traefik router rules
3. It extracts hostnames from `Host()` rules (e.g., ``Host(`app.example.com`)``)
4. For each hostname, it creates or updates an A record in Netcup DNS pointing to the host's IP

## Prerequisites

- Docker with access to the Docker socket
- A Netcup account with API access enabled
- Netcup API credentials (Customer Number, API Key, API Password)

## Usage

> [!IMPORTANT]
> I recommend setting the `DRY_RUN` variable to `true` initially to test your setup without making actual DNS changes. Also you should set the `HOST_IP` variable to your public IP address to ensure the IP will always be correct.

### Docker Compose (Recommended)

```yaml
services:
  docker-traefik-netcup-companion:
    image: ghcr.io/alex289/docker-traefik-netcup-companion:latest
    container_name: docker-traefik-netcup-companion
    restart: unless-stopped
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
    environment:
      - NC_CUSTOMER_NUMBER=12345
      - NC_API_KEY=your_api_key
      - NC_API_PASSWORD=your_api_password
      - DOCKER_FILTER_LABEL=traefik.enable=true
      - DRY_RUN=true
      - HOST_IP=your_host_ip
```

### Docker Run

```bash
docker run -d \
  --name docker-traefik-netcup-companion \
  --restart unless-stopped \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  -e NC_CUSTOMER_NUMBER=12345 \
  -e NC_API_KEY=your_api_key \
  -e NC_API_PASSWORD=your_api_password \
  -e DOCKER_FILTER_LABEL=traefik.enable=true \
  -e DRY_RUN=true \
  -e HOST_IP=your_host_ip \
  ghcr.io/alex289/docker-traefik-netcup-companion:latest
```

## Configuration

The application is configured via environment variables:

| Variable | Required | Description |
|----------|----------|-------------|
| `NC_CUSTOMER_NUMBER` | Yes | Your Netcup customer number |
| `NC_API_KEY` | Yes | Your Netcup API key |
| `NC_API_PASSWORD` | Yes | Your Netcup API password |
| `HOST_IP` | No | Override IP address for DNS records. If not set, auto-detects the host IP (required when running locally as auto-detection returns private IP) |
| `DOCKER_FILTER_LABEL` | No | Filter containers by label (e.g., `traefik.enable=true`) |
| `NC_DEFAULT_TTL` | No | Default TTL for DNS records (default: 300) |
| `DRY_RUN` | No | Enable dry run mode - logs actions without making actual DNS changes (set to `true` or `1`) |
| `NOTIFICATION_URLS` | No | Comma-separated list of notification webhook URLs in [shoutrrr format](https://shoutrrr.nickfedor.com/v0.13.1/services/overview/) (e.g., `slack://token@channel,discord://token@id`) |

### Advanced Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `NC_MAX_RETRIES` | Maximum number of retry attempts | `3` |
| `NC_INITIAL_BACKOFF_MS` | Initial backoff delay in milliseconds | `1000` |
| `NC_MAX_BACKOFF_MS` | Maximum backoff delay in milliseconds | `30000` |
| `NC_BACKOFF_MULTIPLIER` | Multiplier for exponential backoff | `2.0` |
| `NC_CIRCUIT_BREAKER_THRESHOLD` | Consecutive failures to open circuit | `5` |
| `NC_CIRCUIT_BREAKER_TIMEOUT_SEC` | Wait time before retrying (seconds) | `60` |
| `NC_CIRCUIT_BREAKER_HALF_OPEN_REQS` | Test requests in half-open state | `3` |

### Building from Source

```bash
# Clone the repository
git clone https://github.com/alex289/docker-traefik-netcup-companion.git
cd docker-traefik-netcup-companion

# Build with Docker
docker build -t docker-traefik-netcup-companion .

# Or build locally
go build -o companion ./cmd/companion
```

## Example Traefik Labels

The companion looks for Traefik router rule labels containing `Host()` directives:

```yaml
services:
  myapp:
    image: myapp:latest
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.myapp.rule=Host(`myapp.example.com`)"
      - "traefik.http.routers.myapp.entrypoints=websecure"
      - "traefik.http.routers.myapp.tls.certresolver=letsencrypt"
```

When this container starts, the companion will:
1. Detect the ``Host(`myapp.example.com`)`` rule
2. Extract domain `example.com` and subdomain `myapp`
3. Create an A record: `myapp.example.com` → `<host-ip>`

## Dry Run Mode

Dry run mode allows you to test the companion without making actual DNS changes. This is useful for:
- Testing your configuration before going live
- Verifying which DNS records would be created
- Debugging label detection issues

To enable dry run mode, set the `DRY_RUN` environment variable to `true` or `1`:

```yaml
services:
  docker-traefik-netcup-companion:
    image: ghcr.io/alex289/docker-traefik-netcup-companion:latest
    environment:
      - DRY_RUN=true
      - NC_CUSTOMER_NUMBER=12345
      - NC_API_KEY=your_api_key
      - NC_API_PASSWORD=your_api_password
```

When dry run mode is enabled:
- The companion will detect containers and extract hostnames normally
- It will log which DNS records would be created/updated
- No actual API calls to Netcup will be made
- Log messages will be prefixed with `[DRY RUN]`

## Project Structure

```
.
├── cmd/
│   └── companion/
│       └── main.go          # Application entry point
├── internal/
│   ├── config/
│   │   └── config.go        # Configuration loading
│   ├── dns/
│   │   └── manager.go       # DNS record management
│   ├── docker/
│   │   └── watcher.go       # Docker event watching
│   └── netcup/
│       └── netcup.go        # Netcup API client
├── docker-compose.yml
├── Dockerfile
├── go.mod
└── README.md
```

## Getting Netcup API Credentials

1. Log in to your [Netcup Customer Control Panel (CCP)](https://www.customercontrolpanel.de/)
2. Navigate to "Master Data" → "API"
3. Generate or view your API password
4. Your customer number is displayed in the top right corner

## Troubleshooting

### Permission Denied for Docker Socket

Make sure the Docker socket is mounted read-only and accessible:

```yaml
volumes:
  - /var/run/docker.sock:/var/run/docker.sock:ro
```

### DNS Records Not Created

1. Check the logs: `docker logs docker-traefik-netcup-companion`
2. Verify your Netcup credentials are correct
3. Ensure the domain is managed in your Netcup account
4. Check that the container has the correct Traefik labels
5. Check if circuit breaker is open (see logs for "circuit breaker" messages)

### Container Not Detected

If using `DOCKER_FILTER_LABEL`, make sure your container has the matching label:

```yaml
labels:
  - "traefik.enable=true"
```

### API Rate Limiting or Timeouts

The companion includes automatic retry logic and circuit breaker protection. If you see rate limit errors:

1. Check the logs for retry and backoff messages
2. Consider adjusting retry configuration (see [docs/RELIABILITY.md](docs/RELIABILITY.md))
3. Reduce the frequency of container starts/stops if possible

For more troubleshooting related to reliability features, see [docs/RELIABILITY.md](docs/RELIABILITY.md).

## License

MIT License
