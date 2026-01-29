"""Model loading utilities for ML signal generation."""

from __future__ import annotations

import logging
from functools import lru_cache
from pathlib import Path
from typing import List

import numpy as np
import pandas as pd

logger = logging.getLogger(__name__)


@lru_cache(maxsize=8)
def _load_joblib_model(path: str):
    try:
        import joblib
    except Exception as exc:  # pragma: no cover - optional dependency
        raise RuntimeError("joblib is required for loading joblib models") from exc

    return joblib.load(path)


@lru_cache(maxsize=4)
def _load_onnx_session(path: str):
    try:
        import onnxruntime as ort
    except Exception as exc:  # pragma: no cover - optional dependency
        raise RuntimeError("onnxruntime is required for ONNX inference") from exc

    return ort.InferenceSession(path)


def _predict_with_joblib(model, features: np.ndarray) -> np.ndarray:
    if hasattr(model, "predict_proba"):
        probs = model.predict_proba(features)
        return probs[:, -1]
    return model.predict(features)


def _predict_with_onnx(session, features: np.ndarray) -> np.ndarray:
    input_name = session.get_inputs()[0].name
    outputs = session.run(None, {input_name: features.astype(np.float32)})
    result = outputs[0]
    if result.ndim == 2 and result.shape[1] > 1:
        return result[:, -1]
    return result.ravel()


def predict_signal_series(
    bars: pd.DataFrame,
    model_path: str,
    feature_columns: List[str],
) -> pd.Series:
    """Generate ML signal series for the provided bars.

    Returns a float series aligned with bars index.
    """
    model_file = Path(model_path)
    if not model_file.exists():
        logger.warning("ML model not found: %s", model_path)
        return pd.Series([np.nan] * len(bars))

    missing = [c for c in feature_columns if c not in bars.columns]
    if missing:
        logger.warning("ML feature columns missing: %s", missing)
        return pd.Series([np.nan] * len(bars))

    features = bars[feature_columns].astype(np.float32).values
    try:
        if model_file.suffix.lower() == ".onnx":
            session = _load_onnx_session(str(model_file))
            preds = _predict_with_onnx(session, features)
        else:
            model = _load_joblib_model(str(model_file))
            preds = _predict_with_joblib(model, features)
    except Exception as exc:
        logger.warning("ML prediction failed: %s", exc)
        return pd.Series([np.nan] * len(bars))

    return pd.Series(preds, index=bars.index)