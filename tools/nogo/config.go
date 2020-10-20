// Copyright 2019 The gVisor Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package nogo

import (
	"regexp"
)

// GroupName is a named group.
type GroupName string

// AnalyzerName is a named analyzer.
type AnalyzerName string

// Group represents a named collection of files.
type Group struct {
	// Name is the short name for the group.
	Name GroupName `yaml:"name"`

	// Regex is the full position regular expression.
	Regex string `yaml:"regex"`

	// Default is the default match behavior.
	Default bool `yaml:"default"`
}

// ItemConfig is an (Analyzer,Group) configuration.
type ItemConfig struct {
	// Exclude are analyzer exclusions.
	Exclude []string `yaml:"exclude,omitempty"`

	// Suppress are analyzer suppressions.
	Suppress []string `yaml:"suppress,omitempty"`
}

func (i *ItemConfig) merge(other *ItemConfig) {
	i.Exclude = append(i.Exclude, other.Exclude...)
	i.Suppress = append(i.Suppress, other.Suppress...)
}

func (i *ItemConfig) shouldReport(fullPos, msg string) bool {
	for _, excludePath := range i.Exclude {
		if ok, _ := regexp.MatchString(excludePath, fullPos); ok {
			return false
		}
	}
	for _, suppressMsg := range i.Suppress {
		if ok, _ := regexp.MatchString(suppressMsg, msg); ok {
			return false
		}
	}
	return true
}

// AnalyzerConfig is the configuration for a single analyzers.
//
// This map is keyed by individual Group names, to allow for different
// configurations depending on what Group the file belongs to.
type AnalyzerConfig map[GroupName]*ItemConfig

func (a AnalyzerConfig) merge(other AnalyzerConfig) {
	// Merge all the groups.
	for name, gc := range other {
		old, ok := a[name]
		if !ok {
			a[name] = gc // Not configured in a.
			continue
		}
		old.merge(gc)
	}
}

func (a AnalyzerConfig) shouldReport(groupConfig *Group, fullPos, msg string) bool {
	gc, ok := a[groupConfig.Name]
	if !ok {
		return groupConfig.Default
	}
	// Note that if a section appears for a particular group
	// for a particular analyzer, then it will now be enabled,
	// and the group default no longer applies.
	return gc.shouldReport(fullPos, msg)
}

// Config is a nogo configuration.
type Config struct {
	// Prefixes defines a set of regular expressions that
	// are standard "prefixes", so that files can be grouped
	// and specific rules applied to individual groups.
	Groups []Group `yaml:"groups"`

	// Global is the global analyzer config.
	Global AnalyzerConfig `yaml:"global"`

	// Analyzers are individual analyzer configurations. The
	// key for each analyzer is the name of the analyzer. The
	// value is either a boolean (enable/disable), or a map to
	// the groups above.
	Analyzers map[AnalyzerName]AnalyzerConfig `yaml:"analyzers"`
}

// Merge merges two configurations.
func (c *Config) Merge(other *Config) {
	// Merge all groups.
	for _, g := range other.Groups {
		// Is there a matching group? If yes, we just delete
		// it. This will preserve the order provided in the
		// overriding file, even if it differs.
		for i := 0; i < len(c.Groups); i++ {
			if g.Name == c.Groups[i].Name {
				copy(c.Groups[i:], c.Groups[i+1:])
				c.Groups = c.Groups[:len(c.Groups)-1]
				break
			}
		}
		c.Groups = append(c.Groups, g)
	}

	// Merge global configurations.
	c.Global.merge(other.Global)

	// Merge all analyzer configurations.
	for name, ac := range other.Analyzers {
		old, ok := c.Analyzers[name]
		if !ok {
			c.Analyzers[name] = ac // No analyzer in original config.
			continue
		}
		old.merge(ac)
	}
}

// ShouldReport returns true iff the finding should match the Config.
func (c *Config) ShouldReport(finding Finding) bool {
	fullPos := finding.Position.String()

	// Find the matching group.
	var groupConfig *Group
	for i := 0; i < len(c.Groups); i++ {
		if ok, _ := regexp.MatchString(c.Groups[i].Regex, fullPos); ok {
			groupConfig = &c.Groups[i]
			break
		}
	}

	// If there is no group matching this path, then
	// we default to accept the finding. This will be
	// called only when there is no an appropriate
	// global configuration that matches the finding.
	if groupConfig == nil {
		return true
	}

	// Suppress via global rule?
	if !c.Global.shouldReport(groupConfig, fullPos, finding.Message) {
		return false
	}

	// Try the analyzer config.
	ac, ok := c.Analyzers[finding.Category]
	if !ok {
		return groupConfig.Default
	}
	return ac.shouldReport(groupConfig, fullPos, finding.Message)
}
