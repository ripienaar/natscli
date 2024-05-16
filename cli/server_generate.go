// Copyright 2024 The NATS Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cli

import (
	"fmt"
	"github.com/choria-io/fisk"
	au "github.com/nats-io/natscli/internal/auth"
	"github.com/nats-io/natscli/internal/scaffold"
	iu "github.com/nats-io/natscli/internal/util"
	"net/url"
	"strings"
)

type serverGenerateCmd struct {
	source string
	target string
}

func configureServerGenerateCommand(srv *fisk.CmdClause) {
	c := &serverGenerateCmd{}

	gen := srv.Command("generate", `Generate server configurations`).Hidden().Alias("gen").Action(c.generateAction)
	gen.Arg("target", "Write the output to a specific location").Required().StringVar(&c.target)
	gen.Flag("source", "Fetch the configuration bundle from a file or URL").Required().StringVar(&c.source)
}

func (c *serverGenerateCmd) generateAction(_ *fisk.ParseContext) error {
	var b *scaffold.Bundle
	var err error

	if iu.FileExists(c.target) {
		return fmt.Errorf("target directory %s already exist", c.target)
	}

	switch {
	case strings.Contains(c.source, "://"):
		var uri *url.URL
		uri, err = url.Parse(c.source)
		if err != nil {
			return err
		}
		if uri.Scheme == "" {
			return fmt.Errorf("invalid URL %q", c.source)
		}

		b, err = scaffold.FromUrl(uri)
	default:
		b, err = scaffold.FromFile(c.source)
	}
	if err != nil {
		return err
	}

	if b.Requires.Operator {
		auth, err := au.GetAuthBuilder()
		if err != nil {
			return err
		}
		if len(auth.Operators().List()) == 0 {
			return fmt.Errorf("no operator found")
		}
	}

	env := map[string]any{
		"_target": c.target,
		"_source": c.source,
	}

	err = b.Run(c.target, env, opts().Trace)
	if err != nil {
		return err
	}

	return nil
}