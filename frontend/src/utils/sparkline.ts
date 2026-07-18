interface SparklinePoint {
  x: number
  y: number
}

export function buildSmoothSparklinePath(
  values: number[],
  width: number,
  height: number,
  verticalPadding = 10,
): string {
  if (!values.length || width <= 0 || height <= 0) return ''

  const maximum = Math.max(...values, 1)
  const minimum = Math.min(...values)
  const range = maximum - minimum || 1
  const drawableHeight = Math.max(1, height - verticalPadding * 2)
  const points: SparklinePoint[] = values.map((value, index) => ({
    x: (index / Math.max(values.length - 1, 1)) * width,
    y: height - verticalPadding - ((value - minimum) / range) * drawableHeight,
  }))

  if (points.length === 1) {
    const y = round(points[0].y)
    return `M 0 ${y} L ${width} ${y}`
  }

  let path = `M ${round(points[0].x)} ${round(points[0].y)}`
  for (let index = 1; index < points.length - 1; index += 1) {
    const point = points[index]
    const next = points[index + 1]
    const midpointX = (point.x + next.x) / 2
    const midpointY = (point.y + next.y) / 2
    path += ` Q ${round(point.x)} ${round(point.y)} ${round(midpointX)} ${round(midpointY)}`
  }

  const last = points[points.length - 1]
  return `${path} Q ${round(last.x)} ${round(last.y)} ${round(last.x)} ${round(last.y)}`
}

function round(value: number): number {
  return Math.round(value * 100) / 100
}
