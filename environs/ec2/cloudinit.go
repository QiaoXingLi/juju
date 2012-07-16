package ec2

import (
	"fmt"
	"launchpad.net/juju-core/cloudinit"
	"launchpad.net/juju-core/log"
	"launchpad.net/juju-core/state"
	"launchpad.net/juju-core/upstart"
	"path"
	"strings"
)

// machineConfig represents initialization information for a new juju machine.
// Creation of cloudinit data from this struct is largely provider-independent,
// but we'll keep it internal until we need to factor it out.
type machineConfig struct {
	// provisioner specifies whether the new machine will run a provisioning agent.
	provisioner bool

	// machiner specifies whether the new machine will run a machine agent.
	machiner bool

	// zookeeper specifies whether the new machine will run a zookeeper instance.
	zookeeper bool

	// instanceIdAccessor holds bash code that evaluates to the current instance id.
	instanceIdAccessor string

	// providerType identifies the provider type so the host
	// knows which kind of provider to use.
	providerType string

	// stateInfo holds the means for the new instance to communicate with the
	// juju state. Unless the new machine is running zookeeper (Zookeeper is
	// set), there must be at least one zookeeper address supplied.
	stateInfo *state.Info

	// toolsURL is the URL to be used for downloading the juju tools.
	toolsURL string

	// machineId identifies the new machine. It must be non-negative.
	machineId int

	// authorizedKeys specifies the keys that are allowed to
	// connect to the machine (see cloudinit.SSHAddAuthorizedKeys)
	// If no keys are supplied, there can be no ssh access to the node.
	// On a bootstrap machine, that is fatal. On other
	// machines it will mean that the ssh, scp and debug-hooks
	// commands cannot work.
	authorizedKeys string
}

type requiresError string

func (e requiresError) Error() string {
	return "invalid machine configuration: missing " + string(e)
}

func addScripts(c *cloudinit.Config, scripts ...string) {
	for _, s := range scripts {
		c.AddRunCmd(s)
	}
}

func newCloudInit(cfg *machineConfig) (*cloudinit.Config, error) {
	if err := verifyConfig(cfg); err != nil {
		return nil, err
	}
	c := cloudinit.New()

	c.AddSSHAuthorizedKeys(cfg.authorizedKeys)
	if cfg.zookeeper {
		pkgs := []string{
			"default-jre-headless",
			"zookeeper",
			"zookeeperd",
			"libzookeeper-mt2",
		}
		for _, pkg := range pkgs {
			c.AddPackage(pkg)
		}
	}

	addScripts(c,
		"sudo mkdir -p /var/lib/juju",
		"sudo mkdir -p /var/log/juju")

	jujutools := "/var/lib/juju/tools/" + versionDir(cfg.toolsURL)

	// Make a directory for the tools to live in, then fetch the
	// tools and unarchive them into it.
	addScripts(c,
		"bin="+shquote(jujutools),
		"mkdir -p $bin",
		fmt.Sprintf("wget -O - %s | tar xz -C $bin", shquote(cfg.toolsURL)),
	)

	addScripts(c,
		"JUJU_ZOOKEEPER="+shquote(cfg.zookeeperHostAddrs()),
		fmt.Sprintf("JUJU_MACHINE_ID=%d", cfg.machineId),
	)

	debugFlag := ""
	if log.Debug {
		debugFlag = " --debug"
	}

	// zookeeper scripts
	if cfg.zookeeper {
		addScripts(c,
			jujutools+"/jujud initzk"+
				" --instance-id "+cfg.instanceIdAccessor+
				" --env-type "+shquote(cfg.providerType)+
				" --zookeeper-servers localhost"+zkPortSuffix+
				debugFlag,
		)
	}

	if cfg.provisioner {
		svc := upstart.NewService("jujud-provisioning")
		// TODO(rogerpeppe) change upstart.Conf.Cmd to []string so that
		// we don't have to second-guess upstart's quoting rules.
		conf := &upstart.Conf{
			Service: *svc,
			Desc:    "juju provisioning agent",
			Cmd: jujutools + "/jujud provisioning" +
				" --zookeeper-servers " + fmt.Sprintf("'%s'", cfg.zookeeperHostAddrs()) +
				" --log-file /var/log/juju/provision-agent.log" +
				debugFlag,
		}
		cmds, err := conf.InstallCommands()
		if err != nil {
			return nil, fmt.Errorf("cannot make cloudinit provisioning agent upstart script: %v", err)
		}
		addScripts(c, cmds...)
	}

	if cfg.machiner {
		svc := upstart.NewService("jujud-machine-agent")
		// TODO(rogerpeppe) change upstart.Conf.Cmd to []string so that
		// we don't have to second-guess upstart's quoting rules.
		conf := &upstart.Conf{
			Service: *svc,
			Desc:    "juju machine agent",
			Cmd: jujutools + "/jujud machine" +
				fmt.Sprintf(" --zookeeper-servers '%s'", cfg.zookeeperHostAddrs()) +
				fmt.Sprintf(" --machine-id %d", cfg.machineId) +
				" --log-file /var/log/juju/machine-agent.log" +
				debugFlag,
		}
		cmds, err := conf.InstallCommands()
		if err != nil {
			return nil, fmt.Errorf("cannot make cloudinit machine agent upstart script: %v", err)
		}
		addScripts(c, cmds...)
	}

	// general options

	// general options
	c.SetAptUpgrade(true)
	c.SetAptUpdate(true)
	c.SetOutput(cloudinit.OutAll, "| tee -a /var/log/cloud-init-output.log", "")
	return c, nil
}

// versionDir converts a tools URL into a name
// to use as a directory for storing the tools executables in
// by using the last element stripped of its extension.
func versionDir(toolsURL string) string {
	name := path.Base(toolsURL)
	ext := path.Ext(name)
	return name[:len(name)-len(ext)]
}

func (cfg *machineConfig) zookeeperHostAddrs() string {
	var hosts []string
	if cfg.zookeeper {
		hosts = append(hosts, "localhost"+zkPortSuffix)
	}
	if cfg.stateInfo != nil {
		hosts = append(hosts, cfg.stateInfo.Addrs...)
	}
	return strings.Join(hosts, ",")
}

// shquote quotes s so that when read by bash, no metacharacters
// within s will be interpreted as such.
func shquote(s string) string {
	// single-quote becomes single-quote, double-quote, single-quote, double-quote, single-quote
	return `'` + strings.Replace(s, `'`, `'"'"'`, -1) + `'`
}

func verifyConfig(cfg *machineConfig) error {
	if cfg.machineId < 0 {
		return fmt.Errorf("invalid machine configuration: negative machine id")
	}
	if cfg.providerType == "" {
		return requiresError("provider type")
	}
	if cfg.toolsURL == "" {
		return requiresError("tools URL")
	}
	if cfg.zookeeper {
		if cfg.instanceIdAccessor == "" {
			return requiresError("instance id accessor")
		}
	} else {
		if cfg.stateInfo == nil || len(cfg.stateInfo.Addrs) == 0 {
			return requiresError("zookeeper hosts")
		}
	}
	return nil
}
