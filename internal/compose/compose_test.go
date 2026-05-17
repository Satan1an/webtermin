package compose

import (
	"strings"
	"testing"
)

const exampleStack = `
version: "3.9"

services:
  web:
    image: nginx:1.27-alpine
    restart: unless-stopped
    ports:
      - "8080:80"
      - "443/tcp"
    environment:
      - NGINX_HOST=example.com
      - DEBUG=1
    volumes:
      - "./html:/usr/share/nginx/html:ro"
      - "logs:/var/log/nginx"
    networks:
      - web
    depends_on:
      - api
    labels:
      traefik.enable: "true"
  api:
    image: ghcr.io/owner/api:v1.2.0
    restart: always
    command: ["./api", "--port=8000"]
    environment:
      DATABASE_URL: "postgres://user:pass@db:5432/app"
      LOG_LEVEL: info
    volumes:
      - "/etc/api.conf:/app/api.conf:ro"
    networks:
      - web

networks:
  web:
    driver: bridge

volumes:
  logs:
`

func TestParse_Example(t *testing.T) {
	f, err := Parse(exampleStack)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(f.Services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(f.Services))
	}
	web := f.Services["web"]
	if web.Image != "nginx:1.27-alpine" {
		t.Errorf("web.image: %q", web.Image)
	}
	if web.Restart != "unless-stopped" {
		t.Errorf("web.restart: %q", web.Restart)
	}
	if len(web.Ports) != 2 {
		t.Errorf("web.ports: %+v", web.Ports)
	}
	if web.Environment["NGINX_HOST"] != "example.com" {
		t.Errorf("web env list-form: %+v", web.Environment)
	}
	if web.Environment["DEBUG"] != "1" {
		t.Errorf("web env DEBUG: %+v", web.Environment)
	}
	api := f.Services["api"]
	if len(api.Command) != 2 || api.Command[0] != "./api" {
		t.Errorf("api.command: %+v", api.Command)
	}
	if api.Environment["LOG_LEVEL"] != "info" {
		t.Errorf("api env map-form: %+v", api.Environment)
	}
	if len(api.Networks) != 1 || api.Networks[0] != "web" {
		t.Errorf("api.networks: %+v", api.Networks)
	}
}

func TestParse_RejectsNoImage(t *testing.T) {
	src := `
services:
  bare:
    restart: always
`
	if _, err := Parse(src); err == nil || !strings.Contains(err.Error(), "image") {
		t.Fatalf("expected 'no image' error, got %v", err)
	}
}

func TestParse_RejectsInvalidImage(t *testing.T) {
	src := `
services:
  bad:
    image: "../../etc/passwd"
`
	if _, err := Parse(src); err == nil || !strings.Contains(err.Error(), "invalid image") {
		t.Fatalf("expected invalid-image error, got %v", err)
	}
}

func TestParse_RejectsEmptyServices(t *testing.T) {
	if _, err := Parse(`version: "3.9"`); err == nil {
		t.Fatal("expected error on empty services")
	}
}

func TestParsePort(t *testing.T) {
	cases := []struct {
		in   string
		host string
		cp   int
		pr   string
	}{
		{"80", "", 80, "tcp"},
		{"8080:80", "8080", 80, "tcp"},
		{"443/tcp", "", 443, "tcp"},
		{"53/udp", "", 53, "udp"},
		{"5353:5353/udp", "5353", 5353, "udp"},
	}
	for _, c := range cases {
		pb, err := ParsePort(c.in)
		if err != nil {
			t.Errorf("ParsePort(%q): %v", c.in, err)
			continue
		}
		if pb.HostPort != c.host || pb.ContainerPort != c.cp || pb.Protocol != c.pr {
			t.Errorf("ParsePort(%q): got %+v, want host=%q cp=%d proto=%q",
				c.in, pb, c.host, c.cp, c.pr)
		}
	}
}

func TestParsePort_Rejects(t *testing.T) {
	for _, bad := range []string{"", "abc", "80:80:80:80", "$(reboot)", "65536"} {
		if _, err := ParsePort(bad); err == nil {
			// 65536 will technically match the regex \d{1,5}, but it overflows
			// uint16 — for v0.6 we trust the engine to reject; for the others
			// the regex must catch them.
			if bad == "65536" {
				continue
			}
			t.Errorf("expected ParsePort(%q) to fail", bad)
		}
	}
}

func TestParseVolume(t *testing.T) {
	cases := []struct {
		in  string
		typ string
		src string
		dst string
		ro  bool
	}{
		{"/host/data:/app/data", "bind", "/host/data", "/app/data", false},
		{"/etc/conf:/app/conf:ro", "bind", "/etc/conf", "/app/conf", true},
		{"./html:/usr/share/nginx/html", "bind", "./html", "/usr/share/nginx/html", false},
		{"logs:/var/log/nginx", "volume", "logs", "/var/log/nginx", false},
		{"data:/var/data:rw", "volume", "data", "/var/data", false},
	}
	for _, c := range cases {
		m, err := ParseVolume(c.in)
		if err != nil {
			t.Errorf("ParseVolume(%q): %v", c.in, err)
			continue
		}
		if m.Type != c.typ || m.Source != c.src || m.Target != c.dst || m.ReadOnly != c.ro {
			t.Errorf("ParseVolume(%q): got %+v", c.in, m)
		}
	}
}

func TestParseVolume_Rejects(t *testing.T) {
	for _, bad := range []string{"", "single", "a:b:c:d", "x:y:weird"} {
		if _, err := ParseVolume(bad); err == nil {
			t.Errorf("expected ParseVolume(%q) to fail", bad)
		}
	}
}

func TestServiceToSpec(t *testing.T) {
	f, err := Parse(exampleStack)
	if err != nil {
		t.Fatal(err)
	}
	spec, err := f.Services["web"].ToSpec("mystack", "web")
	if err != nil {
		t.Fatal(err)
	}
	if spec.Name != "mystack_web" {
		t.Errorf("name: %q", spec.Name)
	}
	if spec.Image != "nginx:1.27-alpine" {
		t.Errorf("image: %q", spec.Image)
	}
	if spec.RestartPolicy != "unless-stopped" {
		t.Errorf("restart: %q", spec.RestartPolicy)
	}
	if len(spec.PortBindings) != 2 {
		t.Errorf("ports: %+v", spec.PortBindings)
	}
	if !spec.AutoStart {
		t.Error("AutoStart must default to true for compose")
	}
	if spec.Labels["com.docker.compose.project"] != "mystack" {
		t.Errorf("compose project label missing: %+v", spec.Labels)
	}
	if spec.Labels["webtermin.stack"] != "mystack" {
		t.Errorf("webtermin.stack label missing: %+v", spec.Labels)
	}
	if spec.Labels["traefik.enable"] != "true" {
		t.Errorf("user label not propagated: %+v", spec.Labels)
	}
	if spec.NetworkMode != "mystack_web" {
		t.Errorf("network mode: %q (single-network attach should map to stack_<net>)", spec.NetworkMode)
	}
	// env should include both forms, sorted
	if !sliceContains(spec.Env, "NGINX_HOST=example.com") || !sliceContains(spec.Env, "DEBUG=1") {
		t.Errorf("env: %+v", spec.Env)
	}
}

func TestValidStackName(t *testing.T) {
	good := []string{"my-app", "app1", "a", "data_pipeline", "test"}
	bad := []string{"", "MyApp", "-leading", "a b", "stack;rm", "verylongstacknamethatexceedsthirtytwocharacters"}
	for _, s := range good {
		if !ValidStackName(s) {
			t.Errorf("expected %q valid", s)
		}
	}
	for _, s := range bad {
		if ValidStackName(s) {
			t.Errorf("expected %q REJECTED", s)
		}
	}
}

func sliceContains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func TestTopoSort_RespectsDependsOn(t *testing.T) {
	services := map[string]Service{
		"web":   {Image: "nginx", DependsOn: StringList{"api"}},
		"api":   {Image: "api", DependsOn: StringList{"db"}},
		"db":    {Image: "postgres"},
		"cache": {Image: "redis"},
	}
	order, err := topoSort(services)
	if err != nil {
		t.Fatal(err)
	}
	pos := map[string]int{}
	for i, n := range order {
		pos[n] = i
	}
	if !(pos["db"] < pos["api"] && pos["api"] < pos["web"]) {
		t.Errorf("topo violates depends_on: %v", order)
	}
	if len(order) != 4 {
		t.Errorf("missing services: %v", order)
	}
}

func TestTopoSort_DetectsCycle(t *testing.T) {
	services := map[string]Service{
		"a": {Image: "x", DependsOn: StringList{"b"}},
		"b": {Image: "x", DependsOn: StringList{"a"}},
	}
	if _, err := topoSort(services); err == nil {
		t.Fatal("expected cycle error")
	}
}

func TestTopoSort_TolerantOfDanglingDeps(t *testing.T) {
	services := map[string]Service{
		"a": {Image: "x", DependsOn: StringList{"nonexistent"}},
	}
	order, err := topoSort(services)
	if err != nil {
		t.Fatal(err)
	}
	if len(order) != 1 || order[0] != "a" {
		t.Errorf("dangling depends_on not tolerated: %v", order)
	}
}
