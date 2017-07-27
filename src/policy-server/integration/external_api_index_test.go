package integration_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"policy-server/config"
	"policy-server/integration/helpers"
	"strings"

	"code.cloudfoundry.org/cf-networking-helpers/db"
	"code.cloudfoundry.org/cf-networking-helpers/metrics"
	"code.cloudfoundry.org/cf-networking-helpers/testsupport"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("External API Listing Policies", func() {
	var (
		sessions          []*gexec.Session
		conf              config.Config
		policyServerConfs []config.Config
		dbConf            db.Config

		fakeMetron metrics.FakeMetron
	)

	BeforeEach(func() {
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

	Describe("listing policies", func() {
		addPolicy := func(version, body string) {
			resp := helpers.MakeAndDoRequest(
				"POST",
				fmt.Sprintf("http://%s:%d/networking/v0/external/policies", conf.ListenHost, conf.ListenPort),
				map[string]string{"Accept": version},
				strings.NewReader(body),
			)

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			responseString, err := ioutil.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(responseString).To(MatchJSON("{}"))
		}
		BeforeEach(func() {
			addPolicy("1.0.0", `{ "policies": [ {"source": { "id": "app1" }, "destination": { "id": "app2", "protocol": "tcp", "ports": { "start": 1234, "end": 1234 } } } ] }`)
			addPolicy("1.0.0", `{ "policies": [ {"source": { "id": "app3" }, "destination": { "id": "app1", "protocol": "tcp", "ports": { "start": 8080, "end": 8090 } } } ] }`)
			addPolicy("0.0.0", `{ "policies": [ {"source": { "id": "app3" }, "destination": { "id": "app4", "protocol": "tcp", "port": 7777 } } ] }`)
		})

		listPolicies := func(headers map[string]string, queryString, expectedResponse string) {
			resp := helpers.MakeAndDoRequest(
				"GET",
				fmt.Sprintf("http://%s:%d/networking/v0/external/policies%s", conf.ListenHost, conf.ListenPort, queryString),
				headers,
				nil,
			)

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			responseString, err := ioutil.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(responseString).To(MatchJSON(expectedResponse))

			Eventually(fakeMetron.AllEvents, "5s").Should(ContainElement(
				HaveName("PoliciesIndexRequestTime"),
			))
		}

		v1Header := map[string]string{"Accept": "1.0.0"}
		v1Response := `{ "total_policies": 3, "policies": [
		  { "source": { "id": "app1" }, "destination": { "id": "app2", "protocol": "tcp", "ports": { "start": 1234, "end": 1234 } } },
		  { "source": { "id": "app3" }, "destination": { "id": "app1", "protocol": "tcp", "ports": { "start": 8080, "end": 8090 } } },
		  { "source": { "id": "app3" }, "destination": { "id": "app4", "protocol": "tcp", "ports": { "start": 7777, "end": 7777 } } }
		]}`
		v1ResponseFiltered := `{ "total_policies": 2, "policies": [
		  { "source": { "id": "app1" }, "destination": { "id": "app2", "protocol": "tcp", "ports": { "start": 1234, "end": 1234 } } },
		  { "source": { "id": "app3" }, "destination": { "id": "app1", "protocol": "tcp", "ports": { "start": 8080, "end": 8090 } } }
		]}`

		v0Header := map[string]string{"Accept": "0.0.0"}
		v0Response := `{ "total_policies": 2, "policies": [
		  { "source": { "id": "app1" }, "destination": { "id": "app2", "protocol": "tcp", "port": 1234 } },
		  { "source": { "id": "app3" }, "destination": { "id": "app4", "protocol": "tcp", "port": 7777 } }
		]}`
		v0ResponseFiltered := `{ "total_policies": 1, "policies": [
		  { "source": { "id": "app1" }, "destination": { "id": "app2", "protocol": "tcp", "port": 1234 } }
		]}`

		DescribeTable("listing all policies", listPolicies,
			Entry("v1: all", v1Header, "", v1Response),
			Entry("v0: all", v0Header, "", v0Response),
			Entry("no version: all", nil, "", v0Response),
		)

		DescribeTable("listing policies filtered", listPolicies,
			Entry("v1: filtered", v1Header, "?id=app1,app2", v1ResponseFiltered),
			Entry("v0: filtered", v0Header, "?id=app1,app2", v0ResponseFiltered),
			Entry("no version: filtered", nil, "?id=app1,app2", v0ResponseFiltered),
		)
	})
})
