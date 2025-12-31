package docker

import (
	"context"
	"log"
	"regexp"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

type HostInfo struct {
	ContainerID   string
	ContainerName string
	Hostname      string
	Domain        string
	Subdomain     string
}

type Watcher struct {
	client      *client.Client
	filterLabel string
}

func NewWatcher(filterLabel string) (*Watcher, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	return &Watcher{
		client:      cli,
		filterLabel: filterLabel,
	}, nil
}

func (w *Watcher) Close() error {
	return w.client.Close()
}

func (w *Watcher) WatchEvents(ctx context.Context, hostChan chan<- HostInfo) error {
	filterArgs := filters.NewArgs()
	filterArgs.Add("type", "container")
	filterArgs.Add("event", "start")

	eventsChan, errChan := w.client.Events(ctx, events.ListOptions{
		Filters: filterArgs,
	})

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errChan:
			return err
		case event := <-eventsChan:
			w.handleEvent(ctx, event, hostChan)
		}
	}
}

func (w *Watcher) ScanExistingContainers(ctx context.Context) ([]HostInfo, error) {
	var hosts []HostInfo

	filterArgs := filters.NewArgs()
	filterArgs.Add("status", "running")

	containers, err := w.client.ContainerList(ctx, container.ListOptions{
		Filters: filterArgs,
	})
	if err != nil {
		return nil, err
	}

	for _, c := range containers {
		// Check filter label if specified
		if w.filterLabel != "" {
			parts := strings.SplitN(w.filterLabel, "=", 2)
			if len(parts) == 2 {
				if val, ok := c.Labels[parts[0]]; !ok || val != parts[1] {
					continue
				}
			}
		}

		hostInfos := extractHostsFromLabels(c.ID, strings.TrimPrefix(c.Names[0], "/"), c.Labels)
		hosts = append(hosts, hostInfos...)
	}

	return hosts, nil
}

func (w *Watcher) handleEvent(ctx context.Context, event events.Message, hostChan chan<- HostInfo) {
	// Get container details
	containerJSON, err := w.client.ContainerInspect(ctx, event.Actor.ID)
	if err != nil {
		log.Printf("Error inspecting container %s: %v", event.Actor.ID, err)
		return
	}

	labels := containerJSON.Config.Labels

	// Check filter label if specified
	if w.filterLabel != "" {
		parts := strings.SplitN(w.filterLabel, "=", 2)
		if len(parts) == 2 {
			if val, ok := labels[parts[0]]; !ok || val != parts[1] {
				return
			}
		}
	}

	hostInfos := extractHostsFromLabels(event.Actor.ID, containerJSON.Name, labels)
	for _, info := range hostInfos {
		hostChan <- info
	}
}

func extractHostsFromLabels(containerID, containerName string, labels map[string]string) []HostInfo {
	var hosts []HostInfo

	// Regex to match Host rule in Traefik labels
	// Matches patterns like: Host(`example.com`) or Host(`sub.example.com`)
	hostRegex := regexp.MustCompile(`Host\(` + "`" + `([^` + "`" + `]+)` + "`" + `\)`)

	for key, value := range labels {
		// Look for traefik router rule labels
		if strings.Contains(key, "traefik") && strings.Contains(key, ".rule") {
			matches := hostRegex.FindAllStringSubmatch(value, -1)
			for _, match := range matches {
				if len(match) >= 2 {
					hostname := match[1]
					domain, subdomain := splitHostname(hostname)

					hosts = append(hosts, HostInfo{
						ContainerID:   containerID,
						ContainerName: strings.TrimPrefix(containerName, "/"),
						Hostname:      hostname,
						Domain:        domain,
						Subdomain:     subdomain,
					})

					log.Printf("Found host: %s (domain: %s, subdomain: %s) for container %s",
						hostname, domain, subdomain, containerName)
				}
			}
		}
	}

	return hosts
}

// splitHostname splits a hostname into domain and subdomain parts
// e.g., "app.example.com" -> domain: "example.com", subdomain: "app"
// e.g., "example.com" -> domain: "example.com", subdomain: "@"
func splitHostname(hostname string) (domain, subdomain string) {
	parts := strings.Split(hostname, ".")

	if len(parts) < 2 {
		return hostname, "@"
	}

	if len(parts) == 2 {
		return hostname, "@"
	}

	// For hostnames like "app.example.com", domain is "example.com" and subdomain is "app"
	// For hostnames like "sub.app.example.com", domain is "example.com" and subdomain is "sub.app"
	domain = strings.Join(parts[len(parts)-2:], ".")
	subdomain = strings.Join(parts[:len(parts)-2], ".")

	return domain, subdomain
}
