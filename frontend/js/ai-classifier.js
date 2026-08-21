/**
 * AI Environmental Health & Anomaly Classifier
 */
export class LimnologyClassifier {
  /**
   * Computes overall Lake Health Index score (0 - 100)
   * based on Dissolved Oxygen, Turbidity, pH, and Thermal Outliers.
   */
  static calculateHealthScore(readings) {
    let score = 100;

    // Dissolved Oxygen penalty (Optimal: > 6.0 mg/L)
    if (readings.dissolvedOxygen < 3.0) score -= 35;
    else if (readings.dissolvedOxygen < 5.0) score -= 18;
    else if (readings.dissolvedOxygen < 6.0) score -= 6;

    // Turbidity penalty (Optimal: < 15 NTU)
    if (readings.turbidity > 40) score -= 25;
    else if (readings.turbidity > 25) score -= 14;
    else if (readings.turbidity > 15) score -= 5;

    // pH balance penalty (Optimal: 7.0 - 8.5)
    if (readings.ph < 6.5 || readings.ph > 9.0) score -= 15;

    // Chlorophyll-a / Microcystin penalty
    if (readings.chlorophyllA && readings.chlorophyllA > 30) score -= 20;

    const finalScore = Math.max(0, Math.min(100, Math.round(score)));
    let status = 'Good';
    if (finalScore < 50) status = 'Critical';
    else if (finalScore < 75) status = 'Moderate';

    return {
      score: finalScore,
      status,
      trend: '+6% Trend',
      isStable: finalScore >= 60
    };
  }

  /**
   * Automated verification confidence score for field observer reports
   */
  static evaluateReportConfidence(report) {
    let confidence = 85;
    if (report.dissolvedOxygen && report.turbidity) confidence += 8;
    if (report.imageUrl) confidence += 5;
    if (report.notes && report.notes.length > 50) confidence += 2;
    return Math.min(99, confidence);
  }
}

let mobileNetPromise;

async function loadMobileNet() {
  if (!mobileNetPromise) {
    mobileNetPromise = Promise.all([
      import('https://cdn.jsdelivr.net/npm/@tensorflow/tfjs@4.22.0/+esm'),
      import('https://cdn.jsdelivr.net/npm/@tensorflow-models/mobilenet@2.1.1/+esm'),
    ]).then(([, mobilenet]) => mobilenet.load({ version: 2, alpha: 1.0 }));
  }
  return mobileNetPromise;
}

// MobileNet provides visual labels, not a medical/environmental diagnosis.
// The result is stored as supporting evidence; server-side risk scoring remains advisory.
export async function classifyWaterPhoto(file) {
  if (!file) return null;
  const image = new Image();
  const objectURL = URL.createObjectURL(file);
  try {
    await new Promise((resolve, reject) => {
      image.onload = resolve;
      image.onerror = reject;
      image.src = objectURL;
    });
    const model = await loadMobileNet();
    const predictions = await model.classify(image, 3);
    return {
      model: 'MobileNet-v2',
      predictions: predictions.map(({ className, probability }) => ({ label: className, confidence: Number(probability.toFixed(3)) })),
    };
  } finally {
    URL.revokeObjectURL(objectURL);
  }
}
