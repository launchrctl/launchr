# Global configuration

Launchr provides a way to make a global configuration for all actions.  
The global configuration is read from directory `.launchr`. It should have `config.yaml` file.

## Build images

Common images to be used by actions can be provided with the following schema:
```yaml
...
images:
  my/image:version:
    context: ./
    buildfile: test1.Dockerfile
    args:
      arg1: val1
      arg2: val2
    tags:
      - my/image:version2
      - my/image:version3
  my/image2:version:
    context: ./
    buildfile: test2.Dockerfile
    args:
      arg1: val1
      arg2: val2
  my/image3:version: ./
...
```

Image definition search process:
1. Check if image already exists in Docker
2. Check action build definition in `action.yaml`
3. Check global configuration for image name or tags


## Action build hash sum

After first successful build, `actions.sum` file is created in `.launchr` directory.
It stores action directory hash sum of all actions to determine if an image rebuild is required on the next run.

Checking sum difference:
1. Check if `actions.sum` file exists
2. Compare action directory content hash sum with the saved
3. If sum doesn't match, rebuild action image
