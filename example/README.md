# 🌟 Examples

This repository provides **YAML examples** and **code samples** to demonstrate how to configure actions and implement plugins.

---

## 📄 Action YAML Examples

The following YAML examples illustrate configurations for different types of actions:

1. **[Aliases](actions/alias)**  
   Defines shorter, alternative names for actions to simplify usage.

2. **[Arguments and Options](actions/arguments)**  
   Configures action arguments and options that can be supplied during execution.

3. **[Container Image Build Definition](actions/buildargs)**  
   Specifies the container image required for building and running an action.

4. **[Container Environment Variables](actions/envvars)**  
   Configures environment variables accessible in the container when running an action.

Each folder contains YAML files that can serve as templates or examples for specific functionality.

---

## 💻 Code Examples

This section contains examples of Go code to implement plugins and actions:

1. **[Action with Runtime "Plugin"](plugins/action_runtime_plugin)**  
   An example of creating a plugin with a runtime type of `"plugin"`. 
   It includes implementation details for customizing an action.

2. **[Embedded Action](plugins/action_embedfs)**  
   Shows how to embed an action directory in a binary and access it at runtime.  
   Reference the [Action YAML Examples](#-action-yaml-examples) for related configuration examples.

These examples are foundational and can be extended or customized based on project requirements.

---

## 📘 Repository Structure

An overview of the repository’s organization:

- **`actions/` Directory**: Contains YAML configurations for defining and running actions.
- **`plugins/` Directory**: Includes examples of Go code implementations for plugins.
