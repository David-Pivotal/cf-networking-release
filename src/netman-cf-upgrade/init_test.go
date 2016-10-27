package netman_cf_upgrade_test

import (
	"cf-pusher/cf_cli_adapter"
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"
	"time"

	"github.com/cloudfoundry-incubator/cf-test-helpers/helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

const Timeout_Short = 10 * time.Second
const BOSH_DEPLOY_TIMEOUT = 75 * time.Minute

var (
	config     helpers.Config
	boshConfig *BoshConfig
	cli        *cf_cli_adapter.Adapter
)

type BoshConfig struct {
	DirectorURL    string `json:"bosh_director_url"`
	AdminUser      string `json:"bosh_admin_user"`
	AdminPassword  string `json:"bosh_admin_password"`
	DeploymentName string `json:"bosh_deployment_name"`
	DirectorCACert string `json:"bosh_director_ca_cert"`
}

func TestNetmanCfUpgrade(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "NetmanCfUpgrade Suite")
}

var _ = BeforeSuite(func() {
	bytes, err := ioutil.ReadFile(os.Getenv("CONFIG"))
	Expect(err).NotTo(HaveOccurred())
	boshConfig = &BoshConfig{}
	err = json.Unmarshal(bytes, boshConfig)
	Expect(err).NotTo(HaveOccurred(), "Could not unmarshal config file. Make sure it is valid JSON.")
	config = helpers.LoadConfig()
	cli = &cf_cli_adapter.Adapter{CfCliPath: "cf"}
})

func boshDeploy(manifestPath string) {
	bosh("deploy", manifestPath)
}

func boshDeleteDeployment() {
	bosh("delete-deployment")
}

func bosh(args ...string) {
	boshArgs := append([]string{
		"-n",
		"--environment", boshConfig.DirectorURL,
		"--deployment", boshConfig.DeploymentName,
		"--user", boshConfig.AdminUser,
		"--password", boshConfig.AdminPassword,
		"--ca-cert", boshConfig.DirectorCACert}, args...)
	cmd := exec.Command("bosh-cli", boshArgs...)
	sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	Eventually(sess, BOSH_DEPLOY_TIMEOUT).Should(gexec.Exit(0))
}