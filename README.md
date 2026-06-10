# Environmental Impact Analysis with Gemma 4

Kaggle-ready notebook demos for visual environmental impact assessment using Google's latest open multimodal Gemma models.

## Repository contents

- `environmental-impact-analysis-for-everyone-gemma4.ipynb` — local/GPU notebook using the latest official Gemma 4 12B instruction-tuned model.
- `environmental-impact-analysis-for-everyone-gemma4-kaggle.ipynb` — Kaggle-ready notebook with an embedded image and automatic fallback for smaller GPUs.
- `assets/alfred_palmer_smokestacks.jpg` — reference image used by the notebooks.
- `LICENSE` — MIT license for the repository code and notebooks.

## Model choice

The notebooks default to `google/gemma-4-12B-it`.
If the runtime is too small, the Kaggle notebook falls back to `google/gemma-4-E4B-it`.

## Requirements

- Python 3.12+
- GPU recommended
- A Hugging Face account with access to Gemma if required by the model card
- JupyterLab or Kaggle Notebook
- A recent `transformers` build with Gemma 4 multimodal support

## Local quick start

```bash
git clone https://github.com/GhandyP/environmental-impact-gemma4.git
cd environmental-impact-gemma4
python -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
jupyter lab
```

Open one of the notebooks and run all cells.

## Kaggle quick start

1. Create a Kaggle notebook with GPU enabled.
2. Upload or copy `environmental-impact-analysis-for-everyone-gemma4-kaggle.ipynb`.
3. Run all cells.
4. The structured analysis is saved to `/kaggle/working/gemma4_eia_analysis.json`.

## Notes

- The notebooks are inference demos, not fine-tuning pipelines.
- The image analysis output is structured as JSON so it is easy to reuse in downstream code.
- If you want to use a lighter checkpoint locally, switch the model ID to `google/gemma-4-E4B-it`.

## Asset provenance

- `assets/alfred_palmer_smokestacks.jpg` was extracted from the original notebook reference image.
- The embedded EXIF credit is `Library of Congress`.
- The original notebook references the Wikimedia Commons page `https://commons.wikimedia.org/w/index.php?curid=3363860`.
