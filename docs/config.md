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

The build will be done in the following order:
1. Check if image already exists
2. Check action local build definition
3. Search global configuration for image name or tags 
