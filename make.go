package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/pflag"
)

var (
	binary    = pflag.StringSliceP("binary", "b", []string{"controller", "speaker"}, "binaries to act upon")
	action    = pflag.StringSliceP("action", "a", []string{"build"}, "actions to execute")
	arch      = pflag.StringSlice("arch", []string{"amd64"}, "CPU architectures to act upon")
	registry  = pflag.String("registry", "metallb", "docker registry to push to")
	tag       = pflag.StringP("tag", "t", "", "tag to use when building docker images")
	multiarch = pflag.Bool("multiarch", false, "push 'fat' multiarch images in addition to per-arch images")

	validBinaries = map[string]bool{
		"controller":            true,
		"speaker":               true,
		"test-bgp-router":       true,
		"e2etest-mirror-server": true,
	}
	validActions = map[string]func(){
		"build": build,
		"image": image,
		"push":  push,

		"helm":          helm,
		"circleci":      circleci,
		"e2e-manifests": e2eManifests,
	}
	validArchs = map[string]bool{
		"amd64":   true,
		"arm":     true,
		"arm64":   true,
		"ppc64le": true,
		"s390x":   true,
	}
)

func fatal(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
	os.Exit(1)
}

func main() {
	pflag.Parse()
	for _, bin := range *binary {
		if bin == "all" {
			*binary = []string{}
			for bin := range validBinaries {
				*binary = append(*binary, bin)
			}
			sort.Strings(*binary)
			break
		}
		if !validBinaries[bin] {
			fatal("Unknown binary %q", bin)
		}
	}
	for _, act := range *action {
		if validActions[act] == nil {
			fatal("Unknown action %q", act)
		}
	}
	for _, cpu := range *arch {
		if cpu == "all" {
			*arch = []string{}
			for cpu := range validArchs {
				*arch = append(*arch, cpu)
			}
			sort.Strings(*arch)
			break
		}
		if !validArchs[cpu] {
			fatal("Unknown arch %q", cpu)
		}
	}

	for _, act := range *action {
		validActions[act]()
	}
}

func build() {
	commit := outputOf("git", "describe", "--dirty", "--always")
	branch := outputOf("git", "rev-parse", "--abbrev-ref", "HEAD")

	for _, cpu := range *arch {
		for _, bin := range *binary {
			cmd := command(
				"go", "build", "-v",
				"-o", outFile(bin, cpu, bin),
				"-ldflags", fmt.Sprintf("-X go.universe.tf/metallb/internal/version.gitCommit=%s -X go.universe.tf/metallb/internal/version.gitBranch=%s", commit, branch),
				"go.universe.tf/metallb/"+bin,
			)
			cmd.Env = append(
				os.Environ(),
				"CGO_ENABLED=0",
				"GOOS=linux",
				"GOARCH="+cpu,
				"GOARM=6",
			)
			if err := cmd.Run(); err != nil {
				fatal("Build of %q (%s) failed: %s", bin, cpu, err)
			}
		}
	}
}

func image() {
	for _, bin := range *binary {
		for _, cpu := range *arch {
			var img string
			switch cpu {
			case "amd64":
				img = "alpine:latest"
			case "arm":
				img = "arm32v6/alpine:latest"
			case "arm64":
				img = "arm64v8/alpine:latest"
			default:
				img = cpu + "/alpine:latest"
			}

			dockerfile := readFile(bin + "/Dockerfile")
			dockerfile = strings.Replace(dockerfile, "alpine:latest", img, -1)

			if cpu != "amd64" {
				// We build on amd64, so we can't RUN dockerfile commands.
				var lines []string
				for _, line := range strings.Split(dockerfile, "\n") {
					if !strings.HasPrefix(line, "RUN ") {
						lines = append(lines, line)
					}
				}
				dockerfile = strings.Join(lines, "\n")
			}

			existingFile := readFile(outFile(bin, cpu, "Dockerfile"))
			if dockerfile != existingFile {
				writeFile(outFile(bin, cpu, "Dockerfile"), dockerfile)
			}

			dockerName := fmt.Sprintf("%s/%s:%s-%s", *registry, bin, *tag, cpu)
			cmd := command(
				"docker", "build",
				"-t", dockerName,
				".",
			)
			cmd.Dir = buildDir(bin, cpu)
			if err := cmd.Run(); err != nil {
				fatal("Build of %q failed: %v", dockerName, err)
			}
		}
	}
}

func push() {
	for _, bin := range *binary {
		for _, cpu := range *arch {
			dockerName := fmt.Sprintf("%s/%s:%s-%s", *registry, bin, *tag, cpu)
			run("docker", "push", dockerName)
		}

		if *multiarch {
			platforms := []string{}
			for _, cpu := range *arch {
				platforms = append(platforms, "linux/"+cpu)
			}

			multiName := fmt.Sprintf("%s/%s:%s", *registry, bin, *tag)
			run(
				"manifest-tool", "push", "from-args",
				"--platforms", strings.Join(platforms, ","),
				"--template", multiName+"-ARCH",
				"--target", multiName,
			)
		}
	}
}

func helm() {
	manifest := readFile("manifests/namespace.yaml")

	template := outputOf(
		"helm", "template",
		"--namespace", "metallb-system",
		"--set", strings.Join([]string{
			"controller.resources.limits.cpu=100m",
			"controller.resources.limits.memory=100Mi",
			"speaker.resources.limits.cpu=100m",
			"speaker.resources.limits.memory=100Mi",
			"prometheus.scrapeAnnotations=true",
			"existingConfigMap=config",
		}, ","),
		"helm-chart",
	)
	lines := []string{}
processLine:
	for _, line := range strings.Split(template, "\n") {
		line = strings.Replace(line, "RELEASE-NAME-metallb-", "", -1)
		line = strings.Replace(line, "RELEASE-NAME-metallb:", "metallb-system:", -1)

		clean := strings.TrimSpace(line)
		for _, skip := range []string{"heritage:", "release:", "chart:", "# ", "  namespace:"} {
			if strings.HasPrefix(clean, skip) {
				continue processLine
			}
		}

		if strings.HasPrefix(line, "  name: ") && lines[len(lines)-1] == "metadata:" && !strings.HasPrefix(line, "  name: metallb-system") {
			lines = append(lines, "  namespace: metallb-system")
		}
		lines = append(lines, line)
	}

	manifest += strings.Join(lines, "\n") + "\n"

	writeFile("manifests/metallb.yaml", manifest)
}

func circleci() {
	tmpl := template.Must(template.ParseFiles(".circleci/config.yml.tmpl"))
	v := map[string][]string{
		"GoVersions": []string{"1.11"},
		"Binary":     []string{"controller", "speaker", "test-bgp-router"},
	}
	var b bytes.Buffer
	if err := tmpl.Execute(&b, v); err != nil {
		fatal("Rendering circleci config template: %v", err)
	}

	writeFile(".circleci/config.yml", b.String())
}

func e2eManifests() {
	calico := httpGet("https://docs.projectcalico.org/v3.3/getting-started/kubernetes/installation/hosted/rbac-kdd.yaml")
	calico += "---\n"
	calico += httpGet("https://docs.projectcalico.org/v3.3/getting-started/kubernetes/installation/hosted/kubernetes-datastore/calico-networking/1.7/calico.yaml")
	calico = strings.Replace(calico, "192.168.0.0/16", "10.32.0.0/12", -1)
	writeFile("e2etest/manifests/calico.yaml", calico)

	weave := httpGet("https://cloud.weave.works/k8s/net?k8s-version=1.13")
	writeFile("e2etest/manifests/weave.yaml", weave)

	flannel := httpGet("https://raw.githubusercontent.com/coreos/flannel/bc79dd1505b0c8681ece4de4c0d86c5cd2643275/Documentation/kube-flannel.yml")
	flannel = strings.Replace(flannel, "10.244.0.0/16", "10.32.0.0/12", -1)
	writeFile("e2etest/manifests/flannel.yaml", flannel)

	// TODO: cilium, romana, canal, kube-router?
}

// Helpers

func outFile(binary, arch, file string) string {
	return filepath.Join(buildDir(binary, arch), file)
}

func buildDir(binary, arch string) string {
	dir := fmt.Sprintf("build/%s/%s", arch, binary)
	if err := os.MkdirAll(dir, 0750); err != nil {
		fatal("Making build dir %q: %v", dir, err)
	}
	return dir
}

func outputOf(cmd string, args ...string) string {
	bs, err := exec.Command(cmd, args...).Output()
	if err != nil {
		fatal("Running %q: %v", append([]string{cmd}, args...), err)
	}
	return strings.TrimSpace(string(bs))
}

func command(argv0 string, args ...string) *exec.Cmd {
	cmd := exec.Command(argv0, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	fmt.Printf("+ %s\n", strings.Join(cmd.Args, " "))
	return cmd
}

func run(argv0 string, args ...string) {
	cmd := command(argv0, args...)
	if err := cmd.Run(); err != nil {
		fatal("Running %q: %v", strings.Join(cmd.Args, " "), err)
	}
}

func readFile(path string) string {
	bs, err := ioutil.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(bs)
}

func writeFile(path, content string) {
	if err := ioutil.WriteFile(path, []byte(content), 0640); err != nil {
		fatal("Writing %q: %v", path, err)
	}
}

func httpGet(url string) string {
	resp, err := http.Get(url)
	if err != nil {
		fatal("Fetching %q: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fatal("Fetching %q: %v", err)
	}

	return string(body)
}
