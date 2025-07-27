package driver

import (
	"fmt"
	"strings"
	"text/template"
)

func executeTemplate(templateStr string, data any) (string, error) {
	tmpl, err := template.New("script").Parse(templateStr)
	if err != nil {
		return "", err
	}

	var buf strings.Builder
	err = tmpl.Execute(&buf, data)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

// ensure image exists in local registry
const buildahImageEnsureTemplate = `
set -e
cd /action
if [ ! -f "./%s" ]; then
    echo "%s does not exist"
    exit 1
fi
image_status=$(curl -s -I -H "Accept: application/vnd.docker.distribution.manifest.v2+json" \
-H "Accept: application/vnd.docker.distribution.manifest.list.v2+json" \
-H "Accept: application/vnd.oci.image.manifest.v1+json" \
-H "Accept: application/vnd.oci.image.index.v1+json" \
-o /dev/null -w "%%{http_code}" %s)
if [ "$image_status" = "200" ]; then
    echo "image exists"
fi
`

const buildahInitTemplate = `
set -e
echo "Buildah sidecar started"

# Create containers config directory
mkdir -p /etc/containers

# Configure registries
cat > /etc/containers/registries.conf << 'EOF'
unqualified-search-registries = ["docker.io"]

[[registry]]
location = "{{.RegistryURL}}"
{{if .Insecure}}insecure = true{{else}}insecure = false{{end}}
EOF
		
# Test registry connectivity
curl -f {{.RegistryURL}}/v2/ || {
	echo "ERROR: Cannot connect to registry"
	exit 1
}
echo 'Registry connection works'

# Test buildah
buildah version

# Keep running to maintain container availability
while true; do
	sleep 60
	echo "Buildah sidecar still running..."
done
`

// Buildah image build template
const buildahBuildTemplate = `
set -e
cd /action
echo "Starting image build process..."

{{if .BuildArgs}}
# Build arguments:
{{range $key, $value := .BuildArgs}}echo "  --build-arg {{$key}}={{$value}}"
{{end}}
{{end}}

# Build the image
echo "Building image: {{.RegistryURL}}/{{.ImageName}}"
buildah build --layers \
    -t {{.RegistryURL}}/{{.ImageName}} \
    -f {{.Buildfile}} \
{{range $key, $value := .BuildArgs}}    --build-arg {{$key}}="{{$value}}" \
{{end}} . 2>&1 || {
    echo "ERROR: Build failed with exit code $?"
    exit 1
}

echo "Build completed successfully"

# Push to registry
echo "Pushing image to registry..."
{{if .Insecure}}
buildah push --tls-verify=false {{.RegistryURL}}/{{.ImageName}} 2>&1 || {
{{else}}
buildah push {{.RegistryURL}}/{{.ImageName}} 2>&1 || {
{{end}}
    echo "ERROR: Push failed with exit code $?"
    exit 1
}

echo "Build and push completed successfully!"
echo "Image available at:{{.RegistryURL}}/{{.ImageName}}"

# Verify the push
echo "Verifying image was pushed..."
curl -f {{.RegistryURL}}/v2/_catalog || echo "Warning: Could not verify catalog"
`

func (k *k8sRuntime) prepareBuildahInitScript() (string, error) {
	type buildData struct {
		ImageName   string
		RegistryURL string
		Insecure    bool
	}

	data := &buildData{
		RegistryURL: k.crtflags.RegistryURL,
		Insecure:    k.crtflags.RegistryType == RegistryLocal,
	}

	script, err := executeTemplate(buildahInitTemplate, data)
	if err != nil {
		return "", fmt.Errorf("failed to generate init build script: %w", err)
	}

	return script, nil
}

func (k *k8sRuntime) prepareBuildahWorkScript(imageName string) (string, error) {
	// Add build args to template data
	type BuildData struct {
		ImageName   string
		RegistryURL string
		BuildArgs   map[string]*string
		Buildfile   string
		Insecure    bool
	}

	buildData := &BuildData{
		ImageName:   imageName,
		RegistryURL: k.crtflags.RegistryURL,
		BuildArgs:   k.imageOptions.Build.Args,
		Buildfile:   ensureBuildFile(k.imageOptions.Build.Buildfile),
		Insecure:    k.crtflags.RegistryType == RegistryLocal,
	}

	script, err := executeTemplate(buildahBuildTemplate, buildData)
	if err != nil {
		return "", fmt.Errorf("failed to generate image build script: %w", err)
	}

	return script, nil
}
