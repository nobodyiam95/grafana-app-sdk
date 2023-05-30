# Generating Kind Code

Now that we have our kind and schema defined, we want to generate code from them that we can use. In the future, we'll want to re-generate this code whenever we change anything in our `kinds` directory. The SDK provides a command for this: `grafana-app-sdk generate`, but our project init also gave us a make target which will do the same thing, so you can run either. Here, I'm running the make target:
```shell
$ make generate
 * Writing file pkg/generated/resource/issue/cue.mod/module.cue
 * Writing file pkg/generated/resource/issue/issue_lineage.cue
 * Writing file pkg/generated/resource/issue/issue_lineage_gen.go
 * Writing file pkg/generated/resource/issue/issue_metadata_gen.go
 * Writing file pkg/generated/resource/issue/issue_object_gen.go
 * Writing file pkg/generated/resource/issue/issue_schema_gen.go
 * Writing file pkg/generated/resource/issue/issue_spec_gen.go
 * Writing file pkg/generated/resource/issue/issue_status_gen.go
 * Writing file plugin/src/generated/issue_types.gen.ts
 * Writing file definitions/issue.issue-tracker-project.ext.grafana.com.json
```
That's a bunch of files written! Let's tree the directory to understand the structure a bit better.
```shell
$ tree .
.
├── Makefile
├── cmd
│   └── operator
├── definitions
│   └── issue.issue-tracker-project.ext.grafana.com.json
├── go.mod
├── go.sum
├── local
│   ├── Tiltfile
│   ├── additional
│   ├── config.yaml
│   ├── mounted-files
│   │   └── plugin
│   └── scripts
│       ├── cluster.sh
│       └── push_image.sh
├── pkg
│   └── generated
│       └── resource
│           └── issue
│               ├── cue.mod
│               │   └── module.cue
│               ├── issue_lineage.cue
│               ├── issue_lineage_gen.go
│               ├── issue_metadata_gen.go
│               ├── issue_object_gen.go
│               ├── issue_schema_gen.go
│               ├── issue_spec_gen.go
│               └── issue_status_gen.go
├── plugin
│   └── src
│       └── generated
│           └── issue_types.gen.ts
└── kinds
    ├── cue.mod
    │   └── module.cue
    └── issue.cue

19 directories, 19 files
```

So we can now see that all our generated go code lives in the `pkg/generated` package. Since our `target` was `"resource"`, the generated code for `issue` is in the `pkg/generated/resource` package. 
Each `resource`-target kind then lives in a package defined by the name of the kind: in our case, that is `issue`. If we created another kind in our `kinds` directory called "foo", we'd see a `pkg/generated/resource/foo` directory.

If we had a separate `target: "model"` kind, we'd see a `pkg/generated/models` package directory. We'll see that later, in our follow-up, where we extend on the project.

Note that we also have generated TypeScript in our previously-empty `plugin` directory. By convention, the Grafana plugin for your project will live in the `plugin` directory, so here we've got some TypeScript generated in `plugin/src/generated` to use when we start working on the front-end of our plugin.

## Generated Go Code

The package with the largest number of files generated by `make generate` is the `pkg/generated/resource/issue` package. 
This is also the package where all of our generated go code lives (even with multiple kinds, all generated go code will live in `pkg/generated`). 

Let's take a closer look at the list of files:
```shell
$ tree pkg/generated
pkg/generated
└── resource
    └── issue
        ├── cue.mod
        │   └── module.cue
        ├── issue_lineage.cue
        ├── issue_lineage_gen.go
        ├── issue_metadata_gen.go
        ├── issue_object_gen.go
        ├── issue_schema_gen.go
        ├── issue_spec_gen.go
        └── issue_status_gen.go

4 directories, 8 files
```

We can see that along with the generated go files (which are suffixed by `_gen`), we have some generated CUE as well. 
The generated CUE is a copy of the lineage, which must be embedded in our binary for Thema binding. 
The generated code takes care of this embedding and binding. This is done in `issue_lineage_gen.go`. 
Feel free to peruse that file, though familiarity with [Thema](https://github.com/grafana/thema) is advised. 

The exported go types which you will need to use in your projects are defined in `issue_object_gen.go`, `issue_schema_gen.go`, and `issue_spec_gen.go`. 
`issue_object_gen.go` exports a struct called `Object`. This struct implements `resource.Object` in the SDK, and is used for many things that we will get into later as we build out our project. 
`issue_schema_gen.go` exports a function called `Schema()`, which will return an implementation of `resource.Schema` in the SDK, usage of which we'll again get into later. 
Finally, `issue_spec_gen.go` exports the `Spec` type, which is used in our `issue.Object` and is the go type created from our `issue` schema's `spec` field. 
This will always be the latest version of the schema's `spec` (or the `currentVersion`, if defined). 
We also have `issue_metadata_gen.go` and `issue_status_gen.go`. If we recall from the last section, `metadata` and `status` 
are always implicitly included in our schema, and will contain common data for all kinds. 
Opening up either of those files will show you the shape of that common data across all kinds. If you add your own fields explicitly 
with `metadata` or `status`, they will also show up in the `Metadata` or `Status` structs in those files.

## Generated TypeScript Code

```shell
tree plugin
plugin
└── src
    └── generated
        └── issue_types.gen.ts

3 directories, 1 file
```

The generated TypeScript contains an interface built from our schema. TypeScript code is only generated for kinds where `frontend: true`.

### Generated Custom Resource Definitions

Finally, we have the custom resource definition file that describes our `issue` kind as a CRD, which lives in `definitions` by default. 
Note that this is a CRD of the kind, not just the schema, so the CRD will contain all schema versions in the kind. 
This can be used to set up kubernetes as our storage layer for our project.

```shell
tree definitions
definitions
└── issue.unknown-plugin.plugins.grafana.com.json

1 directory, 1 file
```

_NOTE: in the future, using `grafana` as the storage layer will generate CUE for plugin loading that will define the CRD in grafana's API._

So now we have a bunch of generated code, but we still need a project to actually use it in. 
The SDK gives us some tooling to set up our project with boilerplate code, [so let's do that next](04-boilerplate.md).

### Prev: [Defining Our Kinds & Schemas](02-defining-our-kinds.md)
### Next: [Generating Boilerplate](04-boilerplate.md)