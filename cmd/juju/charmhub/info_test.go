// Copyright 2020 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package charmhub

import (
	"github.com/golang/mock/gomock"
	"github.com/juju/charm/v7"
	"github.com/juju/cmd/cmdtesting"
	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"

	"github.com/juju/juju/api/charmhub"
	"github.com/juju/juju/cmd/juju/charmhub/mocks"
)

type infoSuite struct {
	api *mocks.MockInfoCommandAPI
}

var _ = gc.Suite(&infoSuite{})

func (s *infoSuite) TestInitNoArgs(c *gc.C) {
	command := &infoCommand{}
	err := command.Init([]string{})
	c.Assert(err, gc.NotNil)
}

func (s *infoSuite) TestInitSuccess(c *gc.C) {
	command := &infoCommand{}
	err := command.Init([]string{"test"})
	c.Assert(err, jc.ErrorIsNil)
}

func (s *infoSuite) TestRun(c *gc.C) {
	defer s.setUpMocks(c).Finish()
	s.expectInfo()
	command := &infoCommand{api: s.api, charmOrBundle: "test"}
	cmdtesting.InitCommand(command, []string{})
	ctx := commandContextForTest(c)
	err := command.Run(ctx)
	c.Assert(err, jc.ErrorIsNil)
}

func (s *infoSuite) TestRunJSON(c *gc.C) {
	defer s.setUpMocks(c).Finish()
	s.expectInfo()
	command := &infoCommand{api: s.api, charmOrBundle: "test"}
	cmdtesting.InitCommand(command, []string{"--format", "json"})
	ctx := commandContextForTest(c)
	err := command.Run(ctx)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(cmdtesting.Stdout(ctx), gc.Equals, `{"type":"charm","id":"charmCHARMcharmCHARMcharmCHARM01","name":"wordpress","description":"This will install and setup WordPress optimized to run in the cloud.","publisher":"Wordress Charmers","summary":"WordPress is a full featured web blogging tool, this charm deploys it.","series":["bionic","xenial"],"store-url":"https://someurl.com/wordpress","tags":["app","seven"],"charm":{"config":{"Options":{"agility-ratio":{"Type":"float","Description":"A number from 0 to 1 indicating agility.","Default":null},"outlook":{"Type":"string","Description":"No default outlook.","Default":null},"reticulate-splines":{"Type":"boolean","Description":"Whether to reticulate splines on launch, or not.","Default":null},"skill-level":{"Type":"int","Description":"A number indicating skill.","Default":null},"subtitle":{"Type":"string","Description":"An optional subtitle used for the application.","Default":""},"title":{"Type":"string","Description":"A descriptive title used for the application.","Default":"My Title"},"username":{"Type":"string","Description":"The name of the initial account (given admin permissions).","Default":"admin001"}}},"relations":{"provides":{"source":"dummy-token"},"requires":{"sink":"dummy-token"}},"used-by":["wordpress-everlast","wordpress-jorge","wordpress-site"]},"channel-map":{"latest/stable":{"released-at":"2019-12-16T19:44:44.076943+00:00","track":"latest","risk":"stable","revision":16,"size":12042240,"version":"1.0.3"}},"tracks":["latest"]}
`)
}

func (s *infoSuite) TestRunYAML(c *gc.C) {
	defer s.setUpMocks(c).Finish()
	s.expectInfo()
	command := &infoCommand{api: s.api, charmOrBundle: "test"}
	cmdtesting.InitCommand(command, []string{"--format", "yaml"})
	ctx := commandContextForTest(c)
	err := command.Run(ctx)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(cmdtesting.Stdout(ctx), gc.Equals, `
type: charm
id: charmCHARMcharmCHARMcharmCHARM01
name: wordpress
description: This will install and setup WordPress optimized to run in the cloud.
publisher: Wordress Charmers
summary: WordPress is a full featured web blogging tool, this charm deploys it.
series:
- bionic
- xenial
store-url: https://someurl.com/wordpress
tags:
- app
- seven
charm:
  config:
    options:
      agility-ratio:
        type: float
        description: A number from 0 to 1 indicating agility.
      outlook:
        type: string
        description: No default outlook.
      reticulate-splines:
        type: boolean
        description: Whether to reticulate splines on launch, or not.
      skill-level:
        type: int
        description: A number indicating skill.
      subtitle:
        type: string
        description: An optional subtitle used for the application.
        default: ""
      title:
        type: string
        description: A descriptive title used for the application.
        default: My Title
      username:
        type: string
        description: The name of the initial account (given admin permissions).
        default: admin001
  relations:
    provides:
      source: dummy-token
    requires:
      sink: dummy-token
  used-by:
  - wordpress-everlast
  - wordpress-jorge
  - wordpress-site
channel-map:
  latest/stable:
    released-at: "2019-12-16T19:44:44.076943+00:00"
    track: latest
    risk: stable
    revision: 16
    size: 12042240
    version: 1.0.3
tracks:
- latest
`[1:])
}

func (s *infoSuite) setUpMocks(c *gc.C) *gomock.Controller {
	ctrl := gomock.NewController(c)
	s.api = mocks.NewMockInfoCommandAPI(ctrl)
	s.api.EXPECT().Close()
	return ctrl
}

func (s *infoSuite) expectInfo() {
	s.api.EXPECT().Info("test").Return(charmhub.InfoResponse{
		Name:        "wordpress",
		Type:        "charm",
		ID:          "charmCHARMcharmCHARMcharmCHARM01",
		Description: "This will install and setup WordPress optimized to run in the cloud.",
		Publisher:   "Wordress Charmers",
		Summary:     "WordPress is a full featured web blogging tool, this charm deploys it.",
		Tracks:      []string{"latest"},
		Series:      []string{"bionic", "xenial"},
		StoreURL:    "https://someurl.com/wordpress",
		Tags:        []string{"app", "seven"},
		Channels: map[string]charmhub.Channel{
			"latest/stable": {
				ReleasedAt: "2019-12-16T19:44:44.076943+00:00",
				Track:      "latest",
				Risk:       "stable",
				Size:       12042240,
				Revision:   16,
				Version:    "1.0.3",
			}},
		Charm: &charmhub.Charm{
			Subordinate: false,
			Config: &charm.Config{
				Options: map[string]charm.Option{
					"reticulate-splines": {Type: "boolean", Description: "Whether to reticulate splines on launch, or not."},
					"title":              {Type: "string", Description: "A descriptive title used for the application.", Default: "My Title"},
					"subtitle":           {Type: "string", Description: "An optional subtitle used for the application.", Default: ""},
					"outlook":            {Type: "string", Description: "No default outlook."},
					"username":           {Type: "string", Description: "The name of the initial account (given admin permissions).", Default: "admin001"},
					"skill-level":        {Type: "int", Description: "A number indicating skill."},
					"agility-ratio":      {Type: "float", Description: "A number from 0 to 1 indicating agility."},
				},
			},
			Relations: map[string]map[string]string{
				"provides": {"source": "dummy-token"},
				"requires": {"sink": "dummy-token"}},
			UsedBy: []string{
				"wordpress-everlast",
				"wordpress-jorge",
				"wordpress-site",
			},
		},
	}, nil)
}
