# Import adapters/azula-incident/merged-fp16 into Ollama as azula-incident.
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$dir = Join-Path $root "adapters\azula-incident"
$merged = Join-Path $dir "merged-fp16"
$modelfile = Join-Path $dir "Modelfile"
$cfgPath = Join-Path $merged "config.json"

if (-not (Test-Path (Join-Path $merged "model.safetensors"))) {
    Write-Error "Missing $merged\model.safetensors. Extract azula-merged-fp16.zip into adapters\azula-incident\merged-fp16 first."
}
if (-not (Test-Path $modelfile)) {
    Write-Error "Missing $modelfile"
}

python (Join-Path $root "services\trainer\patch_ollama_config.py") $cfgPath

Set-Location $dir
ollama create azula-incident -f Modelfile
ollama list
Write-Host "Model B name: azula-incident  (set LLM_MODEL_B_NAME in .env)"
