// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package specfront

// Shape is the document of a shape spec, in spec/shapes. A callable has at
// most one shape.
type Shape struct {
	Name            Name             `yaml:"name"            schema:"required" doc:"The name of the shape, in lowercase words joined by hyphens."`
	Form            ShapeForm        `yaml:"form"            schema:"required" doc:"The form of the spec, shape."`
	Detected        bool             `yaml:"detected"                          doc:"When true, a detector classifies callables as the shape."`
	Claim           string           `yaml:"claim"           schema:"required" doc:"The assertion of the classification, in neutral vocabulary."`
	Observation     string           `yaml:"observation"     schema:"required" doc:"What a check observes to check the claim."`
	Params          []Param          `yaml:"params"                            doc:"The params of the shape's variant of the shape directive."`
	Bindings        map[Name]Binding `yaml:"bindings"                          doc:"The positions of the callable that fill the parts of the shape, by the name of the part."`
	Falsifiability  string           `yaml:"falsifiability"  schema:"required" doc:"What turns red when someone deletes the subject's handling."`
	Counterexamples Counterexamples  `yaml:"counterexamples" schema:"required" doc:"The inputs that a check covers."`
	Precedence      *Precedence      `yaml:"precedence"                        doc:"The detected shapes that rank before this one."`
}

// Mixin is the document of a mixin spec, in spec/mixins. A callable has
// any number of mixins.
type Mixin struct {
	Name            Name            `yaml:"name"            schema:"required" doc:"The name of the mixin, in lowercase words joined by hyphens."`
	Form            MixinForm       `yaml:"form"            schema:"required" doc:"The form of the spec, mixin."`
	Documentary     bool            `yaml:"documentary"                       doc:"When true, the mixin documents the callable and licenses no check."`
	Claim           string          `yaml:"claim"           schema:"required" doc:"The assertion of the classification, in neutral vocabulary."`
	Observation     string          `yaml:"observation"     schema:"required" doc:"What a check observes to check the claim."`
	Params          []Param         `yaml:"params"                            doc:"The params of the mixin's variant of the mixin directive."`
	Falsifiability  string          `yaml:"falsifiability"  schema:"required" doc:"What turns red when someone deletes the subject's handling."`
	Counterexamples Counterexamples `yaml:"counterexamples" schema:"required" doc:"The inputs that a check covers."`
}

// Contract is the document of a contract spec, in spec/contracts. A
// contract binds callables to the roles of a protocol.
type Contract struct {
	Name            Name               `yaml:"name"            schema:"required" doc:"The name of the contract, in lowercase words joined by hyphens."`
	Form            ContractForm       `yaml:"form"            schema:"required" doc:"The form of the spec, contract."`
	Documentary     bool               `yaml:"documentary"                       doc:"When true, the contract documents its callables and licenses no check."`
	Claim           string             `yaml:"claim"           schema:"required" doc:"The assertion of the classification, in neutral vocabulary."`
	Observation     string             `yaml:"observation"     schema:"required" doc:"What a check observes to check the claim."`
	Params          []Param            `yaml:"params"                            doc:"The params of the contract's variant of the contract directive."`
	Roles           map[Name]RoleArity `yaml:"roles"           schema:"required" doc:"The roles of the protocol, with their arity."`
	Falsifiability  string             `yaml:"falsifiability"  schema:"required" doc:"What turns red when someone deletes the subject's handling."`
	Counterexamples Counterexamples    `yaml:"counterexamples" schema:"required" doc:"The inputs that a check covers."`
}
