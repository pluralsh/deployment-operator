package dns

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/pluralsh/console/go/client"
	"github.com/pluralsh/polly/containers"
	corev1 "k8s.io/api/core/v1"
)

type Prober interface {
	Probe(svc corev1.Service) error
}

type ProbeOptions struct {
	Delay    time.Duration
	Attempts int64
}

type Resolver interface {
	LookupHost(ctx context.Context, host string) ([]string, error)
}

type loadBalancerProber struct {
	resolver Resolver
	fqdn     string
	opts     ProbeOptions
}

func NewLoadBalancerProber(probe *client.TestCaseConfigurationFragment_Loadbalancer_DNSProbe) (Prober, error) {
	if probe == nil {
		return nil, fmt.Errorf("dns probe config must be set")
	}

	if probe.Fqdn == "" {
		return nil, fmt.Errorf("dns probe fqdn must be set")
	}

	delay := time.Duration(0)
	if probe.Delay != nil {
		parsed, err := time.ParseDuration(*probe.Delay)
		if err != nil {
			return nil, fmt.Errorf("invalid dns probe delay %q: %w", *probe.Delay, err)
		}
		delay = parsed
	}

	if delay < 0 {
		return nil, fmt.Errorf("dns probe delay must be non-negative")
	}

	retries := int64(0)
	if probe.Retries != nil {
		retries = *probe.Retries
	}
	if retries < 0 {
		retries = 0
	}

	return &loadBalancerProber{
		resolver: net.DefaultResolver,
		fqdn:     probe.Fqdn,
		opts: ProbeOptions{
			Delay:    delay,
			Attempts: retries + 1,
		},
	}, nil
}

func (in *loadBalancerProber) Probe(svc corev1.Service) error {
	var lastErr error
	timer := time.NewTimer(in.opts.Delay)
	defer timer.Stop()

	addresses, err := in.getAddresses(svc)
	if err != nil {
		return err
	}
	if addresses.Len() == 0 {
		return fmt.Errorf("no load balancer ingress addresses found for %s", svc.Name)
	}

	for attempt := int64(0); attempt < in.opts.Attempts; attempt++ {
		<-timer.C
		lastErr = in.lookup(addresses, in.fqdn)
		if lastErr == nil {
			return nil
		}

		if attempt+1 < in.opts.Attempts {
			timer.Reset(in.opts.Delay)
		}
	}

	return lastErr
}

func (in *loadBalancerProber) lookup(expected containers.Set[string], fqdn string) error {
	resolved, err := in.resolver.LookupHost(context.Background(), fqdn)
	if err != nil {
		return fmt.Errorf("failed to resolve %s: %w", fqdn, err)
	}

	if len(resolved) == 0 {
		return fmt.Errorf("no DNS records resolved for %s", fqdn)
	}

	if !in.hasAddress(resolved, expected) {
		return fmt.Errorf("resolved addresses %v do not match load balancer ingress addresses %v", resolved, expected)
	}

	return nil
}

func (in *loadBalancerProber) getAddresses(svc corev1.Service) (containers.Set[string], error) {
	result := containers.NewSet[string]()
	for _, ingress := range svc.Status.LoadBalancer.Ingress {
		if len(ingress.IP) > 0 {
			result.Add(ingress.IP)
		}

		if len(ingress.Hostname) > 0 {
			addrs, err := in.resolver.LookupHost(context.Background(), ingress.Hostname)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve load balancer hostname %s: %w", ingress.Hostname, err)
			}

			for _, addr := range addrs {
				result.Add(addr)
			}
		}
	}

	return result, nil
}

func (in *loadBalancerProber) hasAddress(resolved []string, expected containers.Set[string]) bool {
	for _, addr := range resolved {
		if expected.Has(addr) {
			return true
		}
	}

	return false
}
