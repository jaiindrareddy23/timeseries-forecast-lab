"""Shared forecasting + evaluation utilities for the demand-forecasting study.

Loads the CSV produced by the Go `datagen` tool, builds calendar/lag
features, fits three forecasting approaches, and scores them on a held-out
test window with MAE / RMSE / MAPE.
"""
from __future__ import annotations

import numpy as np
import pandas as pd
from sklearn.ensemble import GradientBoostingRegressor


def load_series(csv_path: str) -> pd.DataFrame:
    df = pd.read_csv(csv_path, parse_dates=["date"])
    df = df.sort_values("date").reset_index(drop=True)
    df["demand"] = df["demand"].astype(float)
    return df


def train_test_split(df: pd.DataFrame, test_days: int = 90) -> tuple[pd.DataFrame, pd.DataFrame]:
    split_idx = len(df) - test_days
    return df.iloc[:split_idx].reset_index(drop=True), df.iloc[split_idx:].reset_index(drop=True)


def mae(actual: np.ndarray, pred: np.ndarray) -> float:
    return float(np.mean(np.abs(actual - pred)))


def rmse(actual: np.ndarray, pred: np.ndarray) -> float:
    return float(np.sqrt(np.mean((actual - pred) ** 2)))


def mape(actual: np.ndarray, pred: np.ndarray) -> float:
    return float(np.mean(np.abs((actual - pred) / actual)) * 100)


def score(actual: np.ndarray, pred: np.ndarray) -> dict[str, float]:
    return {"MAE": mae(actual, pred), "RMSE": rmse(actual, pred), "MAPE": mape(actual, pred)}


# ---------------------------------------------------------------------------
# Baselines
# ---------------------------------------------------------------------------

def naive_forecast(train: pd.DataFrame, n_test: int) -> np.ndarray:
    """Repeat the last observed value for every day of the horizon."""
    last = train["demand"].iloc[-1]
    return np.full(n_test, last)


def seasonal_naive_forecast(train: pd.DataFrame, n_test: int, season: int = 7) -> np.ndarray:
    """Repeat the value from `season` days ago, cycling through the last season."""
    last_season = train["demand"].iloc[-season:].to_numpy()
    reps = int(np.ceil(n_test / season))
    return np.tile(last_season, reps)[:n_test]


# ---------------------------------------------------------------------------
# Holt-Winters (statsmodels)
# ---------------------------------------------------------------------------

def holt_winters_forecast(train: pd.DataFrame, n_test: int) -> np.ndarray:
    from statsmodels.tsa.holtwinters import ExponentialSmoothing

    model = ExponentialSmoothing(
        train["demand"],
        trend="add",
        damped_trend=True,
        seasonal="add",
        seasonal_periods=7,
        initialization_method="estimated",
    ).fit(optimized=True)
    return model.forecast(n_test).to_numpy()


# ---------------------------------------------------------------------------
# ML regressor on lag + calendar features, recursive multi-step forecast
# ---------------------------------------------------------------------------

LAGS = (1, 7, 14, 28)
ROLL_WINDOWS = (7, 28)


def _calendar_features(dates: pd.Series) -> pd.DataFrame:
    doy = dates.dt.dayofyear.astype(float)
    return pd.DataFrame(
        {
            "dow": dates.dt.dayofweek,
            "month": dates.dt.month,
            "is_weekend": dates.dt.dayofweek.isin([5, 6]).astype(int),
            "doy_sin": np.sin(2 * np.pi * doy / 365.25),
            "doy_cos": np.cos(2 * np.pi * doy / 365.25),
        }
    )


def _build_feature_row(values: np.ndarray, idx: int, date: pd.Timestamp) -> dict:
    row = {"t": idx}
    for lag in LAGS:
        row[f"lag_{lag}"] = values[idx - lag]
    for w in ROLL_WINDOWS:
        row[f"roll_mean_{w}"] = values[idx - w : idx].mean()
    cal = _calendar_features(pd.Series([date]))
    row.update({k: v[0] for k, v in cal.to_dict(orient="list").items()})
    return row


def ml_forecast(train: pd.DataFrame, test: pd.DataFrame, random_state: int = 0) -> np.ndarray:
    """Gradient-boosted trees on lag + calendar features, forecasting the
    test horizon recursively (each day's prediction feeds the lag features
    of later days, as a real deployment would have to)."""
    max_lag = max(LAGS + ROLL_WINDOWS)
    full_dates = pd.concat([train["date"], test["date"]], ignore_index=True)
    known = train["demand"].to_numpy(dtype=float)

    # Build training examples from the training portion only.
    X_rows, y_rows = [], []
    for i in range(max_lag, len(known)):
        X_rows.append(_build_feature_row(known, i, full_dates.iloc[i]))
        y_rows.append(known[i])
    X_train = pd.DataFrame(X_rows)
    y_train = np.array(y_rows)

    model = GradientBoostingRegressor(
        n_estimators=300, max_depth=3, learning_rate=0.05, random_state=random_state
    )
    model.fit(X_train, y_train)

    # Recursive forecast: extend `values` with each prediction so later-step
    # lag/rolling features use previously *forecast* values, not future truth.
    values = np.concatenate([known, np.zeros(len(test))])
    n_train = len(known)
    preds = np.zeros(len(test))
    for step in range(len(test)):
        idx = n_train + step
        row = _build_feature_row(values, idx, full_dates.iloc[idx])
        x = pd.DataFrame([row])[X_train.columns]
        pred = model.predict(x)[0]
        pred = max(pred, 0.0)
        values[idx] = pred
        preds[step] = pred
    return preds
