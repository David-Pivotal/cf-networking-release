package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"policy-server/config"
	"policy-server/integration/helpers"
	"policy-server/api"
	"strings"
	"sync/atomic"

	"code.cloudfoundry.org/cf-networking-helpers/db"
	"code.cloudfoundry.org/cf-networking-helpers/metrics"
	"code.cloudfoundry.org/cf-networking-helpers/testsupport"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("External API", func() {
	var (
		sessions          []*gexec.Session
		conf              config.Config
		policyServerConfs []config.Config
		dbConf            db.Config
		headers           map[string]string

		fakeMetron metrics.FakeMetron
	)

	BeforeEach(func() {
		headers = map[string]string{
			"network-policy-api-version": "1",
		}
		fakeMetron = metrics.NewFakeMetron()

		dbConf = testsupport.GetDBConfig()
		dbConf.DatabaseName = fmt.Sprintf("test_node_%d", GinkgoParallelNode())
		testsupport.CreateDatabase(dbConf)

		template := helpers.DefaultTestConfig(dbConf, fakeMetron.Address(), "fixtures")
		policyServerConfs = configurePolicyServers(template, 2)
		sessions = startPolicyServers(policyServerConfs)
		conf = policyServerConfs[0]
	})

	AfterEach(func() {
		stopPolicyServers(sessions)

		testsupport.RemoveDatabase(dbConf)

		Expect(fakeMetron.Close()).To(Succeed())
	})

	Describe("authentication", func() {
		var makeNewRequest = func(method, route, bodyString string) *http.Request {
			var body io.Reader
			if bodyString != "" {
				body = strings.NewReader(bodyString)
			}
			url := fmt.Sprintf("http://%s:%d/%s", conf.ListenHost, conf.ListenPort, route)
			req, err := http.NewRequest(method, url, body)
			Expect(err).NotTo(HaveOccurred())

			return req
		}

		var TestMissingAuthHeader = func(req *http.Request) {
			By("check that 401 is returned when auth header is missing")
			resp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())

			Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
			responseString, err := ioutil.ReadAll(resp.Body)
			Expect(responseString).To(MatchJSON(`{ "error": "authenticator: missing authorization header"}`))
		}

		var TestBadBearerToken = func(req *http.Request) {
			By("check that 403 is returned when auth header is invalid")
			req.Header.Set("Authorization", "Bearer bad-token")

			resp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())

			Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
			responseString, err := ioutil.ReadAll(resp.Body)
			Expect(responseString).To(MatchJSON(`{ "error": "authenticator: failed to verify token with uaa" }`))
		}

		var _ = DescribeTable("all the routes",
			func(method, route, bodyString string) {
				TestMissingAuthHeader(makeNewRequest(method, route, bodyString))
				TestBadBearerToken(makeNewRequest(method, route, bodyString))
			},
			Entry("POST to policies",
				"POST",
				"networking/v0/external/policies",
				`{ "policies": [ {"source": { "id": "some-app-guid" }, "destination": { "id": "some-other-app-guid", "protocol": "tcp", "ports": { "start": 8090, "end": 8090 } } } ] }`,
			),
			Entry("GET to policies",
				"GET",
				"networking/v0/external/policies",
				``,
			),
			Entry("POST to policies/delete",
				"POST",
				"networking/v0/external/policies/delete",
				`{ "policies": [ {"source": { "id": "some-app-guid" }, "destination": { "id": "some-other-app-guid", "protocol": "tcp", "ports": { "start": 8090, "end": 8090 } } } ] }`,
			),
		)
	})

	Describe("space developer", func() {
		var makeNewRequest = func(method, route, bodyString string) *http.Request {
			var body io.Reader
			if bodyString != "" {
				body = strings.NewReader(bodyString)
			}
			url := fmt.Sprintf("http://%s:%d/%s", conf.ListenHost, conf.ListenPort, route)
			req, err := http.NewRequest(method, url, body)
			Expect(err).NotTo(HaveOccurred())

			req.Header.Set("Authorization", "Bearer space-dev-with-network-write-token")
			return req
		}

		Describe("Create policies", func() {
			var (
				req  *http.Request
				body string
			)
			BeforeEach(func() {
				body = `{ "policies": [ {"source": { "id": "some-app-guid" }, "destination": { "id": "some-other-app-guid", "protocol": "tcp", "ports": { "start": 8090, "end": 8090 } } } ] }`
				req = makeNewRequest("POST", "networking/v0/external/policies", body)
			})

			Context("when space developer self-service is disabled", func() {
				It("succeeds for developers with access to apps and network.write permission", func() {
					resp, err := http.DefaultClient.Do(req)
					Expect(err).NotTo(HaveOccurred())

					Expect(resp.StatusCode).To(Equal(http.StatusOK))
				})

				Context("when they do not have the network.write scope", func() {
					BeforeEach(func() {
						req.Header.Set("Authorization", "Bearer space-dev-token")
					})
					It("returns a 403 with a meaninful error", func() {
						resp, err := http.DefaultClient.Do(req)
						Expect(err).NotTo(HaveOccurred())

						Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
						responseString, err := ioutil.ReadAll(resp.Body)
						Expect(responseString).To(MatchJSON(`{ "error": "authenticator: provided scopes [] do not include allowed scopes [network.admin network.write]"}`))
					})
				})

				Context("when one app is in spaces they do not have access to", func() {
					BeforeEach(func() {
						body = `{ "policies": [ {"source": { "id": "some-app-guid" }, "destination": { "id": "app-guid-not-in-my-spaces", "protocol": "tcp", "ports": { "start": 8090, "end": 8090 } } } ] }`
						req = makeNewRequest("POST", "networking/v0/external/policies", body)
					})
					It("returns a 403 with a meaningful error", func() {
						resp, err := http.DefaultClient.Do(req)
						Expect(err).NotTo(HaveOccurred())

						Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
						responseString, err := ioutil.ReadAll(resp.Body)
						Expect(responseString).To(MatchJSON(`{ "error": "policies-create: one or more applications cannot be found or accessed"}`))
					})
				})
			})

			Context("when space developer self-service is enabled", func() {
				BeforeEach(func() {
					stopPolicyServers(sessions)

					template := helpers.DefaultTestConfig(dbConf, fakeMetron.Address(), "fixtures")
					template.EnableSpaceDeveloperSelfService = true
					policyServerConfs = configurePolicyServers(template, 2)
					sessions = startPolicyServers(policyServerConfs)
					conf = policyServerConfs[0]

					req = makeNewRequest("POST", "networking/v0/external/policies", body)
					req.Header.Set("Authorization", "Bearer space-dev-token")
				})

				It("succeeds for developers with access to apps", func() {
					resp, err := http.DefaultClient.Do(req)
					Expect(err).NotTo(HaveOccurred())

					Expect(resp.StatusCode).To(Equal(http.StatusOK))
				})

				Context("when one app is in spaces they do not have access to", func() {
					BeforeEach(func() {
						body = `{ "policies": [ {"source": { "id": "some-app-guid" }, "destination": { "id": "app-guid-not-in-my-spaces", "protocol": "tcp", "ports": { "start": 8090, "end": 8090 } } } ] }`
						req = makeNewRequest("POST", "networking/v0/external/policies", body)
						req.Header.Set("Authorization", "Bearer space-dev-token")
					})
					It("returns a 403 with a meaningful error", func() {
						resp, err := http.DefaultClient.Do(req)
						Expect(err).NotTo(HaveOccurred())

						Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
						responseString, err := ioutil.ReadAll(resp.Body)
						Expect(responseString).To(MatchJSON(`{ "error": "policies-create: one or more applications cannot be found or accessed"}`))
					})
				})
			})

			It("fails for requests with bodies larger than 10 MB", func() {
				elevenMB := 11 << 20
				bytes := make([]byte, elevenMB, elevenMB)

				req := makeNewRequest("POST", "networking/v0/external/policies", string(bytes))
				resp, err := http.DefaultClient.Do(req)
				Expect(err).NotTo(HaveOccurred())

				Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
				responseString, err := ioutil.ReadAll(resp.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(responseString).To(MatchJSON(`{"error": "policies-create: failed reading request body"}`))
			})
		})

		Describe("Quotas", func() {
			var (
				req  *http.Request
				body string
			)

			BeforeEach(func() {
				body = `{ "policies": [
				{"source": { "id": "some-app-guid" }, "destination": { "id": "some-other-app-guid", "protocol": "tcp", "ports": { "start": 8090, "end": 8090 } } },
				{"source": { "id": "some-app-guid" }, "destination": { "id": "another-app-guid", "protocol": "udp", "ports": { "start": 7070, "end": 7070 } } }
				] }`
				req = makeNewRequest("POST", "networking/v0/external/policies", body)
			})
			It("rejects requests to add policies above the quota", func() {
				By("adding the maximum allowed policies")
				resp, err := http.DefaultClient.Do(req)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				By("seeing that adding another policy fails")
				body = `{ "policies": [
				{"source": { "id": "some-app-guid" }, "destination": { "id": "yet-another-other-app-guid", "protocol": "tcp", "ports": { "start": 9000, "end": 9000 } } }
				] }`
				req = makeNewRequest("POST", "networking/v0/external/policies", body)
				resp, err = http.DefaultClient.Do(req)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
				responseString, err := ioutil.ReadAll(resp.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(responseString).To(MatchJSON(`{"error": "policies-create: policy quota exceeded"}`))

				By("deleting a policy")
				body = `{ "policies": [
				{"source": { "id": "some-app-guid" }, "destination": { "id": "another-app-guid", "protocol": "udp", "ports": { "start": 7070, "end": 7070 } } }
				] }`
				req = makeNewRequest("POST", "networking/v0/external/policies/delete", body)
				resp, err = http.DefaultClient.Do(req)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				By("seeing that adding another policy succeeds")
				body = `{ "policies": [
				{"source": { "id": "some-app-guid" }, "destination": { "id": "yet-another-other-app-guid", "protocol": "tcp", "ports": { "start": 9000, "end": 9000 } } }
				] }`
				req = makeNewRequest("POST", "networking/v0/external/policies", body)
				resp, err = http.DefaultClient.Do(req)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
			})
		})

		Describe("Delete policies", func() {
			var req *http.Request
			BeforeEach(func() {
				body := `{ "policies": [ {"source": { "id": "some-app-guid" }, "destination": { "id": "some-other-app-guid", "protocol": "tcp", "ports": { "start": 8090, "end": 8090 } } } ] }`
				req = makeNewRequest("POST", "networking/v0/external/policies/delete", body)
			})
			It("succeeds for developers with access to apps and network.write permission", func() {
				resp, err := http.DefaultClient.Do(req)
				Expect(err).NotTo(HaveOccurred())

				Expect(resp.StatusCode).To(Equal(http.StatusOK))
			})

			Context("when they do not have the network.write scope", func() {
				BeforeEach(func() {
					req.Header.Set("Authorization", "Bearer space-dev-token")
				})
				It("returns a 403 with a meaninful error", func() {
					resp, err := http.DefaultClient.Do(req)
					Expect(err).NotTo(HaveOccurred())

					Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
					responseString, err := ioutil.ReadAll(resp.Body)
					Expect(responseString).To(MatchJSON(`{ "error": "authenticator: provided scopes [] do not include allowed scopes [network.admin network.write]"}`))
				})
			})
			Context("when one app is in spaces they do not have access to", func() {
				BeforeEach(func() {
					body := `{ "policies": [ {"source": { "id": "some-app-guid" }, "destination": { "id": "app-guid-not-in-my-spaces", "protocol": "tcp", "ports": { "start": 8090, "end": 8090 } } } ] }`
					req = makeNewRequest("POST", "networking/v0/external/policies/delete", body)
				})
				It("returns a 403 with a meaningful error", func() {
					resp, err := http.DefaultClient.Do(req)
					Expect(err).NotTo(HaveOccurred())

					Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
					responseString, err := ioutil.ReadAll(resp.Body)
					Expect(responseString).To(MatchJSON(`{ "error": "delete-policies: one or more applications cannot be found or accessed"}`))
				})
			})
		})

		Describe("List policies", func() {
			var req *http.Request
			BeforeEach(func() {
				req = makeNewRequest("GET", "networking/v0/external/policies", "")
			})

			Context("when there are no policies", func() {
				It("succeeds", func() {
					resp, err := http.DefaultClient.Do(req)
					Expect(err).NotTo(HaveOccurred())

					Expect(resp.StatusCode).To(Equal(http.StatusOK))
					responseString, err := ioutil.ReadAll(resp.Body)
					Expect(responseString).To(MatchJSON(`{
					"total_policies": 0,
					"policies": []
				}`))
				})
			})

			Context("when there are policies in spaces the user does not belong to", func() {
				BeforeEach(func() {
					policies := []api.Policy{}
					for i := 0; i < 150; i++ {
						policies = append(policies, api.Policy{
							Source: api.Source{ID: "live-app-1-guid"},
							Destination: api.Destination{ID: fmt.Sprintf("not-in-space-app-%d-guid", i),
								Ports: api.Ports{
									Start: 8090,
									End:   8090,
								},
								Protocol: "tcp",
							},
						})
					}
					policies = append(policies, api.Policy{
						Source: api.Source{ID: "live-app-1-guid"},
						Destination: api.Destination{ID: "live-app-2-guid",
							Ports: api.Ports{
								Start: 8090,
								End:   8090,
							},
							Protocol:                    "tcp",
						},
					})

					body := map[string][]api.Policy{
						"policies": policies,
					}
					bodyBytes, err := json.Marshal(body)
					Expect(err).NotTo(HaveOccurred())

					req = makeNewRequest("POST", "networking/v0/external/policies", string(bodyBytes))
					req.Header.Set("Authorization", "Bearer valid-token")
					_, err = http.DefaultClient.Do(req)
					Expect(err).NotTo(HaveOccurred())
				})

				It("does not return those policies", func() {
					req = makeNewRequest("GET", "networking/v0/external/policies", "")
					resp, err := http.DefaultClient.Do(req)
					Expect(err).NotTo(HaveOccurred())

					Expect(resp.StatusCode).To(Equal(http.StatusOK))
					responseString, err := ioutil.ReadAll(resp.Body)
					expectedResp := `{
						"total_policies": 1,
						"policies": [ {"source": { "id": "live-app-1-guid" }, "destination": { "id": "live-app-2-guid", "protocol": "tcp", "ports": { "start": 8090, "end": 8090 }}} ]
					}`
					Expect(responseString).To(MatchJSON(expectedResp))
				})
			})

			Context("when they do not have the network.write scope", func() {
				BeforeEach(func() {
					req.Header.Set("Authorization", "Bearer space-dev-token")
				})
				It("returns a 403 with a meaningful error", func() {
					resp, err := http.DefaultClient.Do(req)
					Expect(err).NotTo(HaveOccurred())

					Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
					responseString, err := ioutil.ReadAll(resp.Body)
					Expect(responseString).To(MatchJSON(`{ "error": "authenticator: provided scopes [] do not include allowed scopes [network.admin network.write]"}`))
				})
			})
		})
	})

	Context("when there are concurrent create requests", func() {
		It("remains consistent", func() {
			policiesRoute := "external/policies"
			add := func(policy api.Policy) {
				requestBody, _ := json.Marshal(map[string]interface{}{
					"policies": []api.Policy{policy},
				})
				resp := helpers.MakeAndDoRequest("POST", policyServerUrl(policiesRoute, policyServerConfs), headers, bytes.NewReader(requestBody))
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
				responseString, err := ioutil.ReadAll(resp.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(responseString).To(MatchJSON("{}"))
			}

			nPolicies := 100
			policies := []interface{}{}
			for i := 0; i < nPolicies; i++ {
				appName := fmt.Sprintf("some-app-%x", i)
				policies = append(policies, api.Policy{
					Source: api.Source{ID: appName},
					Destination: api.Destination{
						ID:       appName,
						Protocol: "tcp",
						Ports: api.Ports{
							Start: 1234,
							End:   1234,
						},
					},
				})
			}

			parallelRunner := &testsupport.ParallelRunner{
				NumWorkers: 4,
			}
			By("adding lots of policies concurrently")
			var nAdded int32
			parallelRunner.RunOnSlice(policies, func(policy interface{}) {
				add(policy.(api.Policy))
				atomic.AddInt32(&nAdded, 1)
			})
			Expect(nAdded).To(Equal(int32(nPolicies)))

			By("getting all the policies")
			resp := helpers.MakeAndDoRequest("GET", policyServerUrl(policiesRoute, policyServerConfs), headers, nil)
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			responseBytes, err := ioutil.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			var policiesResponse struct {
				TotalPolicies int             `json:"total_policies"`
				Policies      []api.Policy `json:"policies"`
			}
			Expect(json.Unmarshal(responseBytes, &policiesResponse)).To(Succeed())

			Expect(policiesResponse.TotalPolicies).To(Equal(nPolicies))

			By("verifying all the policies are present")
			for _, policy := range policies {
				Expect(policiesResponse.Policies).To(ContainElement(policy))
			}

			By("verify tags")
			tagsRoute := "external/tags"
			resp = helpers.MakeAndDoRequest("GET", policyServerUrl(tagsRoute, policyServerConfs), headers, nil)
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			responseBytes, err = ioutil.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			var tagsResponse struct {
				Tags []api.Tag `json:"tags"`
			}
			Expect(json.Unmarshal(responseBytes, &tagsResponse)).To(Succeed())
			Expect(tagsResponse.Tags).To(HaveLen(nPolicies))
		})
	})

	Context("when these are concurrent create and delete requests", func() {
		It("remains consistent", func() {
			baseUrl := fmt.Sprintf("http://%s:%d", conf.ListenHost, conf.ListenPort)
			policiesUrl := fmt.Sprintf("%s/networking/v0/external/policies", baseUrl)
			policiesDeleteUrl := fmt.Sprintf("%s/networking/v0/external/policies/delete", baseUrl)

			do := func(method, url string, policy api.Policy) {
				requestBody, _ := json.Marshal(map[string]interface{}{
					"policies": []api.Policy{policy},
				})
				resp := helpers.MakeAndDoRequest(method, url, headers, bytes.NewReader(requestBody))
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
				responseString, err := ioutil.ReadAll(resp.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(responseString).To(MatchJSON("{}"))
			}

			nPolicies := 100
			policies := []interface{}{}
			for i := 0; i < nPolicies; i++ {
				appName := fmt.Sprintf("some-app-%x", i)
				policies = append(policies, api.Policy{
					Source:      api.Source{ID: appName},
					Destination: api.Destination{
						ID: appName,
						Protocol: "tcp",
						Ports: api.Ports{
							Start: 8090,
							End:   8090,
						},
					},
				})
			}

			parallelRunner := &testsupport.ParallelRunner{
				NumWorkers: 4,
			}
			toDelete := make(chan (interface{}), nPolicies)

			go func() {
				parallelRunner.RunOnSlice(policies, func(policy interface{}) {
					p := policy.(api.Policy)
					do("POST", policiesUrl, p)
					toDelete <- p
				})
				close(toDelete)
			}()

			var nDeleted int32
			parallelRunner.RunOnChannel(toDelete, func(policy interface{}) {
				p := policy.(api.Policy)
				do("POST", policiesDeleteUrl, p)
				atomic.AddInt32(&nDeleted, 1)
			})

			Expect(nDeleted).To(Equal(int32(nPolicies)))

			resp := helpers.MakeAndDoRequest("GET", policiesUrl, headers, nil)
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			responseBytes, err := ioutil.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			var policiesResponse struct {
				TotalPolicies int             `json:"total_policies"`
				Policies      []api.Policy `json:"policies"`
			}
			Expect(json.Unmarshal(responseBytes, &policiesResponse)).To(Succeed())

			Expect(policiesResponse.TotalPolicies).To(Equal(0))
		})
	})

	Describe("adding policies", func() {
		It("responds with 200 and a body of {} and we can see it in the list", func() {
			body := strings.NewReader(`{ "policies": [ {"source": { "id": "some-app-guid" }, "destination": { "id": "some-other-app-guid", "protocol": "tcp", "ports": { "start": 8090, "end": 8090 } } } ] }`)
			resp := helpers.MakeAndDoRequest(
				"POST",
				fmt.Sprintf("http://%s:%d/networking/v0/external/policies", conf.ListenHost, conf.ListenPort),
				headers,
				body,
			)

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			responseString, err := ioutil.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(responseString).To(MatchJSON("{}"))

			resp = helpers.MakeAndDoRequest(
				"GET",
				fmt.Sprintf("http://%s:%d/networking/v0/external/policies", conf.ListenHost, conf.ListenPort),
				headers,
				nil,
			)

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			responseString, err = ioutil.ReadAll(resp.Body)
			Expect(responseString).To(MatchJSON(`{
				"total_policies": 1,
				"policies": [
				{ "source": { "id": "some-app-guid" }, "destination": { "id": "some-other-app-guid", "protocol": "tcp", "ports": { "start": 8090, "end": 8090 } } }
				]}`))

			Eventually(fakeMetron.AllEvents, "5s").Should(ContainElement(
				HaveName("CreatePoliciesRequestTime"),
			))
			Eventually(fakeMetron.AllEvents, "5s").Should(ContainElement(
				HaveName("StoreCreateSuccessTime"),
			))
		})
		Context("when using the ports field to specify one port", func() {
			It("responds with 200 and a body of {} and we can see it in the list", func() {
				body := strings.NewReader(`{ "policies": [ {"source": { "id": "some-app-guid" }, "destination": { "id": "some-other-app-guid", "protocol": "tcp", "ports": { "start": 8090, "end": 8090 } } } ] }`)
				resp := helpers.MakeAndDoRequest(
					"POST",
					fmt.Sprintf("http://%s:%d/networking/v0/external/policies", conf.ListenHost, conf.ListenPort),
					headers,
					body,
				)

				Expect(resp.StatusCode).To(Equal(http.StatusOK))
				responseString, err := ioutil.ReadAll(resp.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(responseString).To(MatchJSON("{}"))

				resp = helpers.MakeAndDoRequest(
					"GET",
					fmt.Sprintf("http://%s:%d/networking/v0/external/policies", conf.ListenHost, conf.ListenPort),
					headers,
					nil,
				)

				Expect(resp.StatusCode).To(Equal(http.StatusOK))
				responseString, err = ioutil.ReadAll(resp.Body)
				Expect(responseString).To(MatchJSON(`{
				"total_policies": 1,
				"policies": [
				{ "source": { "id": "some-app-guid" }, "destination": { "id": "some-other-app-guid", "protocol": "tcp", "ports": { "start": 8090, "end": 8090 } } }
				]}`))

				Eventually(fakeMetron.AllEvents, "5s").Should(ContainElement(
					HaveName("CreatePoliciesRequestTime"),
				))
				Eventually(fakeMetron.AllEvents, "5s").Should(ContainElement(
					HaveName("StoreCreateSuccessTime"),
				))
			})

		})

		Context("when the protocol is invalid", func() {
			It("gives a helpful error", func() {
				body := strings.NewReader(`{ "policies": [ {"source": { "id": "some-app-guid" }, "destination": { "id": "some-other-app-guid", "protocol": "nope", "ports": { "start": 8090, "end": 8090 } } } ] }`)
				resp := helpers.MakeAndDoRequest(
					"POST",
					fmt.Sprintf("http://%s:%d/networking/v0/external/policies", conf.ListenHost, conf.ListenPort),
					headers,
					body,
				)

				Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
				responseString, err := ioutil.ReadAll(resp.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(responseString).To(MatchJSON(`{ "error": "policies-create: invalid destination protocol, specify either udp or tcp" }`))
			})
		})

		Context("when the port is invalid", func() {
			It("gives a helpful error", func() {
				body := strings.NewReader(`{ "policies": [ {"source": { "id": "some-app-guid" }, "destination": { "id": "some-other-app-guid", "protocol": "tcp", "ports": { "start": 0, "end": 3454 } } } ] }`)
				resp := helpers.MakeAndDoRequest(
					"POST",
					fmt.Sprintf("http://%s:%d/networking/v0/external/policies", conf.ListenHost, conf.ListenPort),
					headers,
					body,
				)

				Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
				responseString, err := ioutil.ReadAll(resp.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(responseString).To(MatchJSON(`{ "error": "policies-create: invalid start port 0, must be in range 1-65535" }`))
			})
		})
	})

	Describe("cleanup policies", func() {
		BeforeEach(func() {
			body := strings.NewReader(`{ "policies": [
				{"source": { "id": "live-app-1-guid" }, "destination": { "id": "live-app-2-guid", "protocol": "tcp", "ports": { "start": 8080, "end": 8080 } } },
				{"source": { "id": "live-app-2-guid" }, "destination": { "id": "live-app-2-guid", "protocol": "tcp", "ports": { "start": 9999, "end": 9999 } } },
				{"source": { "id": "live-app-1-guid" }, "destination": { "id": "dead-app", "protocol": "tcp", "ports": { "start": 3333, "end": 3333 }} }
				]} `)

			resp := helpers.MakeAndDoRequest(
				"POST",
				fmt.Sprintf("http://%s:%d/networking/v0/external/policies", conf.ListenHost, conf.ListenPort),
				headers,
				body,
			)
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

		})

		It("responds with a 200 and lists all stale policies", func() {
			resp := helpers.MakeAndDoRequest(
				"POST",
				fmt.Sprintf("http://%s:%d/networking/v0/external/policies/cleanup", conf.ListenHost, conf.ListenPort),
				headers,
				nil,
			)

			stalePoliciesStr := `{
				"total_policies":1,
				"policies": [
				{"source": { "id": "live-app-1-guid" }, "destination": { "id": "dead-app", "protocol": "tcp", "ports": { "start": 3333, "end": 3333 } } }
				 ]}
				`

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			bodyBytes, _ := ioutil.ReadAll(resp.Body)
			Expect(bodyBytes).To(MatchJSON(stalePoliciesStr))
			Eventually(fakeMetron.AllEvents, "5s").Should(ContainElement(
				HaveName("CleanupRequestTime"),
			))
			Eventually(fakeMetron.AllEvents, "5s").Should(ContainElement(
				HaveName("StoreDeleteSuccessTime"),
			))
		})
	})

	Describe("listing policies", func() {
		Context("when providing a list of ids as a query parameter", func() {
			It("responds with a 200 and lists all policies which contain one of those ids", func() {
				resp := helpers.MakeAndDoRequest(
					"POST",
					fmt.Sprintf("http://%s:%d/networking/v0/external/policies", conf.ListenHost, conf.ListenPort),
					headers,
					strings.NewReader(`{ "policies": [
						{"source": { "id": "app1" }, "destination": { "id": "app2", "protocol": "tcp", "ports": { "start": 8080, "end": 8080 } } },
						{"source": { "id": "app3" }, "destination": { "id": "app1", "protocol": "tcp", "ports": { "start": 9999, "end": 9999 } } },
						{"source": { "id": "app3" }, "destination": { "id": "app4", "protocol": "tcp", "ports": { "start": 3333, "end": 3333 } } }
					]}
					`),
				)

				Expect(resp.StatusCode).To(Equal(http.StatusOK))
				responseString, err := ioutil.ReadAll(resp.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(responseString).To(MatchJSON("{}"))

				resp = helpers.MakeAndDoRequest(
					"GET",
					fmt.Sprintf("http://%s:%d/networking/v0/external/policies?id=app1,app2", conf.ListenHost, conf.ListenPort),
					headers,
					nil,
				)

				Expect(resp.StatusCode).To(Equal(http.StatusOK))
				responseString, err = ioutil.ReadAll(resp.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(responseString).To(MatchJSON(`{
					"total_policies": 2,
					"policies": [
					{"source": { "id": "app1" }, "destination": { "id": "app2", "protocol": "tcp", "ports": { "start": 8080, "end": 8080 } } },
				 {"source": { "id": "app3" }, "destination": { "id": "app1", "protocol": "tcp", "ports": { "start": 9999, "end": 9999 }} }
				 ]}
				`))

				By("emitting metrics about durations")
				Eventually(fakeMetron.AllEvents, "5s").Should(ContainElement(
					HaveName("PoliciesIndexRequestTime"),
				))
				Eventually(fakeMetron.AllEvents, "5s").Should(ContainElement(
					HaveName("StoreAllSuccessTime"),
				))
			})
		})
	})

	Describe("deleting policies", func() {
		BeforeEach(func() {
			resp := helpers.MakeAndDoRequest(
				"POST",
				fmt.Sprintf("http://%s:%d/networking/v0/external/policies", conf.ListenHost, conf.ListenPort),
				headers,
				strings.NewReader(`{ "policies": [ {"source": { "id": "some-app-guid" }, "destination": { "id": "some-other-app-guid", "protocol": "tcp", "ports": { "start": 8090, "end": 8090 } } } ] }`),
			)

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			responseString, err := ioutil.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(responseString).To(MatchJSON("{}"))
		})

		Context("when all of the deletes succeed", func() {
			It("responds with 200 and a body of {} and we can see it is removed from the list", func() {

				body := strings.NewReader(`{ "policies": [ {"source": { "id": "some-app-guid" }, "destination": { "id": "some-other-app-guid", "protocol": "tcp", "ports": { "start": 8090, "end": 8090 } } } ] }`)

				response := helpers.MakeAndDoRequest(
					"POST",
					fmt.Sprintf("http://%s:%d/networking/v0/external/policies/delete", conf.ListenHost, conf.ListenPort),
					headers,
					body,
				)

				Expect(response.StatusCode).To(Equal(http.StatusOK))
				responseString, err := ioutil.ReadAll(response.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(responseString).To(MatchJSON(`{}`))

				response = helpers.MakeAndDoRequest(
					"GET",
					fmt.Sprintf("http://%s:%d/networking/v0/external/policies", conf.ListenHost, conf.ListenPort),
					headers,
					nil,
				)

				Expect(response.StatusCode).To(Equal(http.StatusOK))
				responseString, err = ioutil.ReadAll(response.Body)
				Expect(responseString).To(MatchJSON(`{
					"total_policies": 0,
					"policies": []
				}`))

				By("emitting metrics about durations")
				Eventually(fakeMetron.AllEvents, "5s").Should(ContainElement(
					HaveName("DeletePoliciesRequestTime"),
				))
				Eventually(fakeMetron.AllEvents, "5s").Should(ContainElement(
					HaveName("StoreDeleteSuccessTime"),
				))
			})
			It("still works when a single port is set in the request", func() {

				body := strings.NewReader(`{ "policies": [ {"source": { "id": "some-app-guid" }, "destination": { "id": "some-other-app-guid", "protocol": "tcp", "ports": { "start": 8090, "end": 8090 } } } ] }`)

				response := helpers.MakeAndDoRequest(
					"POST",
					fmt.Sprintf("http://%s:%d/networking/v0/external/policies/delete", conf.ListenHost, conf.ListenPort),
					headers,
					body,
				)

				Expect(response.StatusCode).To(Equal(http.StatusOK))
				responseString, err := ioutil.ReadAll(response.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(responseString).To(MatchJSON(`{}`))

				response = helpers.MakeAndDoRequest(
					"GET",
					fmt.Sprintf("http://%s:%d/networking/v0/external/policies", conf.ListenHost, conf.ListenPort),
					headers,
					nil,
				)

				Expect(response.StatusCode).To(Equal(http.StatusOK))
				responseString, err = ioutil.ReadAll(response.Body)
				Expect(responseString).To(MatchJSON(`{
					"total_policies": 0,
					"policies": []
				}`))
			})
		})

		Context("when one of the policies to delete does not exist", func() {
			It("responds with status 200", func() {
				body := strings.NewReader(`{ "policies": [
						{"source": { "id": "some-app-guid" }, "destination": { "id": "some-other-app-guid", "protocol": "tcp", "ports": { "start": 8090, "end": 8090 } } },
						{"source": { "id": "some-non-existent-app-guid" }, "destination": { "id": "some-other-app-guid", "protocol": "tcp", "ports": { "start": 8090, "end": 8090 } } }
					] }`)

				response := helpers.MakeAndDoRequest(
					"POST",
					fmt.Sprintf("http://%s:%d/networking/v0/external/policies/delete", conf.ListenHost, conf.ListenPort),
					headers,
					body,
				)

				Expect(response.StatusCode).To(Equal(http.StatusOK))
				responseString, err := ioutil.ReadAll(response.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(responseString).To(MatchJSON(`{}`))

			})
		})
	})

	Describe("port ranges", func() {
		It("allows the user to create, list and delete policies with port ranges", func() {
			body := strings.NewReader(`{ "policies": [
			{ "source": {"id": "some-app-guid"},
			 "destination": {"id": "some-other-app-guid", "protocol": "tcp", "ports": {"start": 5000, "end": 6000}}}]}`)
			resp := helpers.MakeAndDoRequest(
				"POST",
				fmt.Sprintf("http://%s:%d/networking/v0/external/policies", conf.ListenHost, conf.ListenPort),
				headers,
				body,
			)

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			responseString, err := ioutil.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(responseString).To(MatchJSON("{}"))

			resp = helpers.MakeAndDoRequest(
				"GET",
				fmt.Sprintf("http://%s:%d/networking/v0/external/policies", conf.ListenHost, conf.ListenPort),
				headers,
				nil,
			)

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			responseString, err = ioutil.ReadAll(resp.Body)
			Expect(responseString).To(MatchJSON(`{
				"total_policies": 1,
				"policies": [
				{ "source": { "id": "some-app-guid" }, "destination": { "id": "some-other-app-guid", "protocol": "tcp", "ports": { "start": 5000, "end": 6000 } } }
				]}`))

			//TODO add delete test
		})
	})

	Describe("listing tags", func() {
		BeforeEach(func() {
			body := strings.NewReader(`{ "policies": [
			{"source": { "id": "some-app-guid" }, "destination": { "id": "some-other-app-guid", "protocol": "tcp", "ports": { "start": 8090, "end": 8090 } } },
			{"source": { "id": "some-app-guid" }, "destination": { "id": "another-app-guid", "protocol": "udp", "ports": { "start": 6666, "end": 6666 } } },
			{"source": { "id": "another-app-guid" }, "destination": { "id": "some-app-guid", "protocol": "tcp", "ports": { "start": 3333, "end": 3333 } } }
			] }`)
			resp := helpers.MakeAndDoRequest(
				"POST",
				fmt.Sprintf("http://%s:%d/networking/v0/external/policies", conf.ListenHost, conf.ListenPort),
				headers,
				body,
			)

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			responseString, err := ioutil.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(responseString).To(MatchJSON("{}"))
		})

		It("returns a list of application guid to tag mapping", func() {
			By("listing the current tags")
			resp := helpers.MakeAndDoRequest(
				"GET",
				fmt.Sprintf("http://%s:%d/networking/v0/external/tags", conf.ListenHost, conf.ListenPort),
				headers,
				nil,
			)

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			responseString, err := ioutil.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(responseString).To(MatchJSON(`{ "tags": [
				{ "id": "some-app-guid", "tag": "01" },
				{ "id": "some-other-app-guid", "tag": "02" },
				{ "id": "another-app-guid", "tag": "03" }
			] }`))

			By("reusing tags that are no longer in use")
			body := strings.NewReader(`{ "policies": [ {"source": { "id": "some-app-guid" }, "destination": { "id": "some-other-app-guid", "protocol": "tcp", "ports": { "start": 8090, "end": 8090 } } } ] }`)
			resp = helpers.MakeAndDoRequest(
				"POST",
				fmt.Sprintf("http://%s:%d/networking/v0/external/policies/delete", conf.ListenHost, conf.ListenPort),
				headers,
				body,
			)

			body = strings.NewReader(`{ "policies": [ {"source": { "id": "some-app-guid" }, "destination": { "id": "yet-another-app-guid", "protocol": "udp", "ports": { "start": 4567, "end": 4567 } } } ] }`)
			resp = helpers.MakeAndDoRequest(
				"POST",
				fmt.Sprintf("http://%s:%d/networking/v0/external/policies", conf.ListenHost, conf.ListenPort),
				headers,
				body,
			)

			resp = helpers.MakeAndDoRequest(
				"GET",
				fmt.Sprintf("http://%s:%d/networking/v0/external/tags", conf.ListenHost, conf.ListenPort),
				headers,
				nil,
			)

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			responseString, err = ioutil.ReadAll(resp.Body)
			Expect(responseString).To(MatchJSON(`{ "tags": [
				{ "id": "some-app-guid", "tag": "01" },
				{ "id": "yet-another-app-guid", "tag": "02" },
				{ "id": "another-app-guid", "tag": "03" }
			] }`))

			Eventually(fakeMetron.AllEvents, "5s").Should(ContainElement(
				HaveName("TagsIndexRequestTime"),
			))
			Eventually(fakeMetron.AllEvents, "5s").Should(ContainElement(
				HaveName("StoreTagsSuccessTime"),
			))
		})
	})

	Describe("uptime", func() {
		It("returns 200 when server is healthy", func() {
			resp := helpers.MakeAndDoRequest(
				"GET",
				fmt.Sprintf("http://%s:%d/", conf.ListenHost, conf.ListenPort),
				headers,
				nil,
			)

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
		})

		Context("when the database is unavailable", func() {
			BeforeEach(func() {
				testsupport.RemoveDatabase(dbConf)
			})

			It("still returns a 200", func() {
				resp := helpers.MakeAndDoRequest(
					"GET",
					fmt.Sprintf("http://%s:%d/", conf.ListenHost, conf.ListenPort),
					headers,
					nil,
				)

				Expect(resp.StatusCode).To(Equal(http.StatusOK))
			})
		})
	})

	Describe("health", func() {
		It("returns 200 when server is healthy", func() {
			resp := helpers.MakeAndDoRequest(
				"GET",
				fmt.Sprintf("http://%s:%d/health", conf.ListenHost, conf.ListenPort),
				headers,
				nil,
			)

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
		})
	})
})
