package acceptance_test

import (
	"cf-pusher/cf_cli_adapter"
	"encoding/json"
	"os/exec"
	"strconv"
	"time"

	"github.com/cloudfoundry-incubator/cf-test-helpers/cf"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

const Timeout_Task_Curl = 1 * time.Minute

type ProxyResponse struct {
	ListenAddresses []string ""
	Port            int
}

var _ = PDescribe("task connectivity on the overlay network", func() {
	Describe("networking policy", func() {
		var (
			prefix  string
			orgName string
			cfCli   *cf_cli_adapter.Adapter
			proxy1  string
			proxy2  string
		)

		BeforeEach(func() {
			cfCli = &cf_cli_adapter.Adapter{CfCliPath: "cf"}
			prefix = testConfig.Prefix

			orgName = prefix + "task-org"
			Expect(cf.Cf("create-org", orgName).Wait(Timeout_Push)).To(gexec.Exit(0))
			Expect(cf.Cf("target", "-o", orgName).Wait(Timeout_Push)).To(gexec.Exit(0))

			spaceName := prefix + "space"
			Expect(cf.Cf("create-space", spaceName, "-o", orgName).Wait(Timeout_Push)).To(gexec.Exit(0))
			Expect(cf.Cf("target", "-o", orgName, "-s", spaceName).Wait(Timeout_Push)).To(gexec.Exit(0))

			proxy1 = "proxy-task-connectivity-1"
			proxy2 = "proxy-task-connectivity-2"

			pushProxy(proxy1)
			pushProxy(proxy2)

			cfCli.AllowAccess(proxy1, proxy2, 8080, "tcp")

		})

		AfterEach(func() {
			Expect(cf.Cf("delete-org", orgName, "-f").Wait(Timeout_Push)).To(gexec.Exit(0))
		})

		It("allows app instances to talk to tasks", func(done Done) {
			// Run proxy1 task that talks to itself through proxy2 via router
		})

		It("allows tasks to talk to app instances", func(done Done) {
			By("getting the overlay ip of proxy2")
			cmd := exec.Command("curl", "--fail", proxy2+".bosh-lite.com")
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess, 5*time.Second).Should(gexec.Exit(0))
			var proxy2Response ProxyResponse
			Expect(json.Unmarshal(sess.Out.Contents(), &proxy2Response)).To(Succeed())
			Expect(proxy2Response.ListenAddresses).To(HaveLen(2))

			By("Checking that the task associated with proxy1 can connect to proxy2")
			Expect(cf.Cf("run-task", proxy1, `
			while true; do
				if curl --fail "`+proxy2Response.ListenAddresses[1]+`:`+strconv.Itoa(proxy2Response.Port)+`" ; then
					exit 0
				fi
			done;
			exit 1
			`).Wait(5 * time.Second)).To(gexec.Exit(0))

			Eventually(func() *gbytes.Buffer {
				return cf.Cf("tasks", proxy1).Wait(5 * time.Second).Out
			}, Timeout_Task_Curl).Should(gbytes.Say("SUCCEEDED"))

			close(done)
		}, 30*60 /* <-- overall spec timeout in seconds */)
	})
})
