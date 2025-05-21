# Global configuration

Launchr provides a way to make a global configuration for all actions.
The global configuration is read from directory `.launchr`. It should have `config.yaml` file.  
If the application was build with a different name, the directory will be named accordingly `.my_app_name`.

## Table of contents

* [Container runtime](#container-runtime)
* [Modify action names after discovery](#modify-action-names-after-discovery)
* [Build images](#build-images)
* [Action image build hash sums](#action-image-build-hash-sums)

## Container runtime

To change the default container runtime:

```yaml
# ...
container:
  runtime: kubernetes
# ...
```

If not specified, the action will use **Docker** as a default runtime.

## Modify action names after discovery

It's possible to replace parts of the original action ID to receive prettier naming.

```yaml
# ...
launchrctl:
  actions_naming:
    - search: ".replaceme."
      replace: "."
    - search: "_"
      replace: "-"
# ...
```

In the given example, if an action is located in `foo/replaceme/bar_buz/actions/fred/action.yaml`
  * Before: `foo.replaceme.bar_buz:fred`
  * After: `foo.bar-buz:fred`

## Build images

Common images to be used by actions can be provided with the following schema:
```yaml
# ...
images:
  my/image:version:
    context: ./
    buildfile: test1.Dockerfile
    args:
      arg1: val1
      arg2: val2
  my/image2:version:
    context: ./
    buildfile: test2.Dockerfile
    args:
      arg1: val1
      arg2: val2
# ...
```

Image definition search process:
1. Check if an image already exists in a container registry
2. Check action build definition in `action.yaml`
3. Check global configuration for image name or tags


## Action image build hash sums

After first successful build, `actions.sum` file is created in `.launchr` directory.
It stores a hash sum of an action directory for all actions to determine if an image rebuild is required on the next run.

Checking sum difference:
1. Check if `actions.sum` file exists
2. Compare action directory content hash sum with the saved
3. If sum doesn't match, rebuild action image
