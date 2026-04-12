//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func setupConsumerEnvWithKey(g Gomega, apiKey string) []string {
	env := []string{
		"A6_CONFIG_DIR=" + GinkgoT().TempDir(),
	}
	_, stderr, err := runA6WithEnv(env, "context", "create", "test",
		"--server", adminURL, "--api-key", apiKey)
	g.Expect(err).NotTo(HaveOccurred(), "failed to create consumer test context: %s", stderr)
	return env
}

func writeConsumerFile(g Gomega, name, body string) string {
	path := filepath.Join(GinkgoT().TempDir(), name)
	g.Expect(os.WriteFile(path, []byte(body), 0o644)).To(Succeed())
	return path
}

func deleteConsumerViaAdminByUsername(g Gomega, username string) {
	resp, err := adminAPI("DELETE", "/apisix/admin/consumers/"+username, nil)
	if err == nil && resp != nil {
		g.Expect(resp.Body.Close()).To(Succeed())
	}
}

func deleteConsumerViaCLIByUsername(env []string, username string) {
	_, _, _ = runA6WithEnv(env, "consumer", "delete", username, "--force")
}

func createConsumerViaCLIFile(g Gomega, env []string, file string) {
	stdout, stderr, err := runA6WithEnv(env, "consumer", "create", "-f", file)
	g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)
}

var _ = Describe("consumer command", Ordered, func() {
	var env []string

	BeforeEach(func() {
		env = setupConsumerEnvWithKey(NewWithT(GinkgoT()), adminKey)
	})

	Describe("create", func() {
		It("creates consumers from JSON and YAML files against the real Admin API", func() {
			g := NewWithT(GinkgoT())
			jsonUsername := "ginkgo-consumer-json"
			yamlUsername := "ginkgo-consumer-yaml"

			deleteConsumerViaCLIByUsername(env, jsonUsername)
			deleteConsumerViaCLIByUsername(env, yamlUsername)
			DeferCleanup(deleteConsumerViaAdminByUsername, g, jsonUsername)
			DeferCleanup(deleteConsumerViaAdminByUsername, g, yamlUsername)

			jsonFile := writeConsumerFile(g, "consumer.json", fmt.Sprintf(`{
  "username": "%s",
  "desc": "ginkgo consumer json",
  "plugins": {
    "key-auth": {
      "key": "ginkgo-json-key"
    }
  }
}`, jsonUsername))
			stdout, stderr, err := runA6WithEnv(env, "consumer", "create", "-f", jsonFile)
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)
			g.Expect(stdout).To(ContainSubstring(jsonUsername))

			resp, apiErr := adminAPI("GET", "/apisix/admin/consumers/"+jsonUsername, nil)
			g.Expect(apiErr).NotTo(HaveOccurred())
			g.Expect(resp.StatusCode).To(Equal(200))
			g.Expect(resp.Body.Close()).To(Succeed())

			yamlFile := writeConsumerFile(g, "consumer.yaml", fmt.Sprintf(`username: %s
desc: ginkgo consumer yaml
plugins:
  key-auth:
    key: ginkgo-yaml-key
`, yamlUsername))
			stdout, stderr, err = runA6WithEnv(env, "consumer", "create", "-f", yamlFile, "--output", "yaml")
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)
			g.Expect(stdout).To(ContainSubstring("username: " + yamlUsername))
		})

		It("preserves required-flag validation and surfaces real API errors", func() {
			g := NewWithT(GinkgoT())

			_, stderr, err := runA6WithEnv(env, "consumer", "create")
			g.Expect(err).To(HaveOccurred())
			g.Expect(stderr).To(ContainSubstring("required flag"))

			invalidFile := writeConsumerFile(g, "consumer-invalid.json", `{
  "username": "invalid-key-auth",
  "plugins": {
    "key-auth": {}
  }
}`)
			_, stderr, err = runA6WithEnv(env, "consumer", "create", "-f", invalidFile)
			g.Expect(err).To(HaveOccurred())
			g.Expect(strings.ToLower(stderr)).To(ContainSubstring("error"))
		})
	})

	Describe("list and export", func() {
		It("renders list output modes and surfaces authentication failures", func() {
			g := NewWithT(GinkgoT())
			username1 := "ginkgo-consumer-list-1"
			username2 := "ginkgo-consumer-list-2"

			deleteConsumerViaCLIByUsername(env, username1)
			deleteConsumerViaCLIByUsername(env, username2)
			DeferCleanup(deleteConsumerViaAdminByUsername, g, username1)
			DeferCleanup(deleteConsumerViaAdminByUsername, g, username2)

			file1 := writeConsumerFile(g, "consumer-list-1.json", fmt.Sprintf(`{
  "username": "%s",
  "desc": "ginkgo consumer alpha",
  "labels": {"env":"test"},
  "plugins": {
    "key-auth": {
      "key": "ginkgo-list-key-1"
    }
  }
}`, username1))
			createConsumerViaCLIFile(g, env, file1)

			file2 := writeConsumerFile(g, "consumer-list-2.json", fmt.Sprintf(`{
  "username": "%s",
  "desc": "ginkgo consumer beta",
  "labels": {"env":"prod"},
  "plugins": {
    "key-auth": {
      "key": "ginkgo-list-key-2"
    }
  }
}`, username2))
			createConsumerViaCLIFile(g, env, file2)

			stdout, stderr, err := runA6WithEnv(env, "consumer", "list", "--output", "table")
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)
			g.Expect(stdout).To(ContainSubstring("USERNAME"))
			g.Expect(stdout).To(ContainSubstring(username1))
			g.Expect(stdout).To(ContainSubstring(username2))

			stdout, stderr, err = runA6WithEnv(env, "consumer", "list", "--output", "json")
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)
			g.Expect(json.Valid([]byte(stdout))).To(BeTrue(), stdout)
			g.Expect(stdout).To(ContainSubstring(username1))
			g.Expect(stdout).To(ContainSubstring(username2))

			stdout, stderr, err = runA6WithEnv(env, "consumer", "list", "--output", "yaml")
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)
			g.Expect(stdout).To(ContainSubstring("username: " + username1))
			g.Expect(stdout).To(ContainSubstring("username: " + username2))

			badEnv := setupConsumerEnvWithKey(g, "invalid-api-key")
			_, stderr, err = runA6WithEnv(badEnv, "consumer", "list")
			g.Expect(err).To(HaveOccurred())
			g.Expect(strings.ToLower(stderr)).To(SatisfyAny(
				ContainSubstring("authentication failed"),
				ContainSubstring("permission denied"),
			))
		})

		It("exports consumers with real label filtering and strips timestamps from output", func() {
			g := NewWithT(GinkgoT())
			username1 := "ginkgo-consumer-export-1"
			username2 := "ginkgo-consumer-export-2"

			deleteConsumerViaCLIByUsername(env, username1)
			deleteConsumerViaCLIByUsername(env, username2)
			DeferCleanup(deleteConsumerViaAdminByUsername, g, username1)
			DeferCleanup(deleteConsumerViaAdminByUsername, g, username2)

			file1 := writeConsumerFile(g, "consumer-export-1.json", fmt.Sprintf(`{
  "username": "%s",
  "desc": "ginkgo consumer export 1",
  "labels": {"env":"test"},
  "plugins": {
    "key-auth": {
      "key": "ginkgo-export-key-1"
    }
  }
}`, username1))
			createConsumerViaCLIFile(g, env, file1)

			file2 := writeConsumerFile(g, "consumer-export-2.json", fmt.Sprintf(`{
  "username": "%s",
  "desc": "ginkgo consumer export 2",
  "labels": {"env":"prod"},
  "plugins": {
    "key-auth": {
      "key": "ginkgo-export-key-2"
    }
  }
}`, username2))
			createConsumerViaCLIFile(g, env, file2)

			stdout, stderr, err := runA6WithEnv(env, "consumer", "export", "--label", "env=test", "--output", "json")
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)
			g.Expect(stdout).To(ContainSubstring(username1))
			g.Expect(stdout).NotTo(ContainSubstring(username2))
			g.Expect(stdout).NotTo(ContainSubstring("create_time"))
			g.Expect(stdout).NotTo(ContainSubstring("update_time"))

			outFile := filepath.Join(GinkgoT().TempDir(), "consumer-export.yaml")
			stdout, stderr, err = runA6WithEnv(env, "consumer", "export", "-f", outFile, "--output", "yaml")
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)

			exported, readErr := os.ReadFile(outFile)
			g.Expect(readErr).NotTo(HaveOccurred())
			g.Expect(string(exported)).To(ContainSubstring(username1))
			g.Expect(string(exported)).To(ContainSubstring(username2))
			g.Expect(string(exported)).NotTo(ContainSubstring("create_time"))
			g.Expect(string(exported)).NotTo(ContainSubstring("update_time"))
		})
	})

	Describe("get, update, and delete", func() {
		It("gets consumers in yaml/json, updates them, and deletes them against the real Admin API", func() {
			g := NewWithT(GinkgoT())
			username := "ginkgo-consumer-lifecycle"

			deleteConsumerViaCLIByUsername(env, username)
			DeferCleanup(deleteConsumerViaAdminByUsername, g, username)

			createFile := writeConsumerFile(g, "consumer-lifecycle.json", fmt.Sprintf(`{
  "username": "%s",
  "desc": "ginkgo consumer lifecycle",
  "plugins": {
    "key-auth": {
      "key": "ginkgo-lifecycle-key"
    }
  }
}`, username))
			createConsumerViaCLIFile(g, env, createFile)

			stdout, stderr, err := runA6WithEnv(env, "consumer", "get", username, "--output", "yaml")
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)
			g.Expect(stdout).To(ContainSubstring("username: " + username))
			g.Expect(stdout).To(ContainSubstring("ginkgo consumer lifecycle"))

			stdout, stderr, err = runA6WithEnv(env, "consumer", "get", username, "--output", "json")
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)
			g.Expect(json.Valid([]byte(stdout))).To(BeTrue(), stdout)
			g.Expect(stdout).To(ContainSubstring(username))

			updateFile := writeConsumerFile(g, "consumer-update.json", `{
  "desc": "ginkgo consumer updated",
  "plugins": {
    "key-auth": {
      "key": "ginkgo-updated-key"
    }
  }
}`)
			stdout, stderr, err = runA6WithEnv(env, "consumer", "update", username, "-f", updateFile)
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)
			g.Expect(stdout).To(ContainSubstring("ginkgo consumer updated"))

			stdout, stderr, err = runA6WithEnv(env, "consumer", "get", username, "--output", "json")
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)
			g.Expect(stdout).To(ContainSubstring("ginkgo consumer updated"))

			stdout, stderr, err = runA6WithEnv(env, "consumer", "delete", username, "--force")
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)
			g.Expect(stdout).To(ContainSubstring("deleted"))

			resp, apiErr := adminAPI("GET", "/apisix/admin/consumers/"+username, nil)
			g.Expect(apiErr).NotTo(HaveOccurred())
			g.Expect(resp.StatusCode).To(Equal(404))
			g.Expect(resp.Body.Close()).To(Succeed())
		})

		It("enforces key-auth through a real route and consumer", func() {
			g := NewWithT(GinkgoT())
			username := "ginkgo-key-auth-user"
			routeID := "ginkgo-consumer-auth-route"

			deleteConsumerViaCLIByUsername(env, username)
			deleteRouteViaCLIByID(env, routeID)
			DeferCleanup(deleteRouteViaAdminByID, g, routeID)
			DeferCleanup(deleteConsumerViaAdminByUsername, g, username)

			consumerFile := writeConsumerFile(g, "consumer-auth.json", fmt.Sprintf(`{
  "username": "%s",
  "plugins": {
    "key-auth": {
      "key": "ginkgo-auth-key"
    }
  }
}`, username))
			createConsumerViaCLIFile(g, env, consumerFile)

			routeFile := writeRouteFile(g, "consumer-auth-route.json", fmt.Sprintf(`{
  "id": "%s",
  "uri": "/ginkgo-consumer-auth/*",
  "plugins": {
    "key-auth": {},
    "proxy-rewrite": {
      "regex_uri": ["^/ginkgo-consumer-auth/(.*)", "/$1"]
    }
  },
  "upstream": {
    "type": "roundrobin",
    "nodes": {
      "httpbin:8080": 1
    }
  }
}`, routeID))
			stdout, stderr, err := runA6WithEnv(env, "route", "create", "-f", routeFile)
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)

			var authResp *http.Response
			for range 10 {
				req, reqErr := http.NewRequest("GET", gatewayURL+"/ginkgo-consumer-auth/get", nil)
				g.Expect(reqErr).NotTo(HaveOccurred())
				req.Header.Set("apikey", "ginkgo-auth-key")
				authResp, err = http.DefaultClient.Do(req)
				g.Expect(err).NotTo(HaveOccurred())
				if authResp.StatusCode == http.StatusOK {
					break
				}
				g.Expect(authResp.Body.Close()).To(Succeed())
				time.Sleep(500 * time.Millisecond)
			}
			g.Expect(authResp).NotTo(BeNil())
			g.Expect(authResp.StatusCode).To(Equal(http.StatusOK))
			g.Expect(authResp.Body.Close()).To(Succeed())

			noAuthResp, callErr := http.Get(gatewayURL + "/ginkgo-consumer-auth/get")
			g.Expect(callErr).NotTo(HaveOccurred())
			g.Expect(noAuthResp.StatusCode).To(Equal(http.StatusUnauthorized))
			g.Expect(noAuthResp.Body.Close()).To(Succeed())
		})

		It("surfaces get and delete not-found behavior and update required-flag validation through the real CLI", func() {
			g := NewWithT(GinkgoT())
			username := "nonexistent-consumer-999"

			deleteConsumerViaCLIByUsername(env, username)
			DeferCleanup(deleteConsumerViaAdminByUsername, g, username)

			_, stderr, err := runA6WithEnv(env, "consumer", "get", username)
			g.Expect(err).To(HaveOccurred())
			g.Expect(strings.ToLower(stderr)).To(ContainSubstring("not found"))

			_, stderr, err = runA6WithEnv(env, "consumer", "update")
			g.Expect(err).To(HaveOccurred())
			g.Expect(stderr).To(ContainSubstring("required flag"))

			_, stderr, err = runA6WithEnv(env, "consumer", "delete", username, "--force")
			g.Expect(err).To(HaveOccurred())
			g.Expect(strings.ToLower(stderr)).To(ContainSubstring("not found"))
		})
	})
})
