<template>
  <div class="force-graph-container" ref="containerRef">
    <svg ref="svgRef" class="force-svg"></svg>
    <div class="graph-legend">
      <div class="legend-item">
        <span class="legend-color" style="background: #ef4444;"></span>
        <span>冲突关系</span>
      </div>
      <div class="legend-item">
        <span class="legend-color" style="background: #f59e0b;"></span>
        <span>覆盖关系</span>
      </div>
      <div class="legend-item">
        <span class="legend-color" style="background: #10b981;"></span>
        <span>互补关系</span>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, watch, onUnmounted } from 'vue'
import * as d3 from 'd3'

const props = defineProps({
  nodes: {
    type: Array,
    default: () => []
  },
  edges: {
    type: Array,
    default: () => []
  }
})

const emit = defineEmits(['node-click'])

const containerRef = ref(null)
const svgRef = ref(null)
let simulation = null
let svg = null
let g = null

const edgeColors = {
  conflict: '#ef4444',
  override: '#f59e0b',
  complement: '#10b981'
}

const nodeColors = {
  permit: '#10b981',
  deny: '#ef4444'
}

const initGraph = () => {
  if (!containerRef.value || !svgRef.value) return

  const container = containerRef.value
  const width = container.clientWidth
  const height = container.clientHeight

  svg = d3.select(svgRef.value)
    .attr('width', width)
    .attr('height', height)

  svg.selectAll('*').remove()

  const gZoom = svg.append('g')
  g = gZoom

  const zoom = d3.zoom()
    .scaleExtent([0.1, 4])
    .on('zoom', (event) => {
      gZoom.attr('transform', event.transform)
    })

  svg.call(zoom)

  const defs = svg.append('defs')

  Object.keys(edgeColors).forEach(type => {
    defs.append('marker')
      .attr('id', `arrow-${type}`)
      .attr('viewBox', '0 -5 10 10')
      .attr('refX', 20)
      .attr('refY', 0)
      .attr('markerWidth', 6)
      .attr('markerHeight', 6)
      .attr('orient', 'auto')
      .append('path')
      .attr('d', 'M0,-5L10,0L0,5')
      .attr('fill', edgeColors[type])
  })

  const link = g.append('g')
    .attr('class', 'links')
    .selectAll('line')
    .data(props.edges)
    .enter()
    .append('line')
    .attr('stroke', d => edgeColors[d.type] || '#999')
    .attr('stroke-width', 2)
    .attr('stroke-opacity', 0.6)
    .attr('marker-end', d => `url(#arrow-${d.type})`)

  const node = g.append('g')
    .attr('class', 'nodes')
    .selectAll('g')
    .data(props.nodes)
    .enter()
    .append('g')
    .attr('cursor', 'pointer')
    .call(d3.drag()
      .on('start', dragstarted)
      .on('drag', dragged)
      .on('end', dragended))

  node.append('circle')
    .attr('r', 18)
    .attr('fill', d => nodeColors[d.effect] || '#6b7280')
    .attr('stroke', '#fff')
    .attr('stroke-width', 2)

  node.append('text')
    .text(d => d.id.substring(0, 10))
    .attr('text-anchor', 'middle')
    .attr('dy', 30)
    .attr('font-size', '10px')
    .attr('fill', '#374151')

  const tooltip = d3.select('body').append('div')
    .attr('class', 'graph-tooltip')
    .style('position', 'absolute')
    .style('padding', '8px 12px')
    .style('background', 'rgba(0,0,0,0.8)')
    .style('color', '#fff')
    .style('border-radius', '4px')
    .style('font-size', '12px')
    .style('pointer-events', 'none')
    .style('opacity', 0)
    .style('z-index', 1000)

  node.on('mouseover', function(event, d) {
    d3.select(this).select('circle')
      .transition()
      .duration(200)
      .attr('r', 22)

    const effectText = d.effect === 'permit' ? '允许' : '拒绝'
    const levelText = { global: '全局', tenant: '租户', project: '项目' }[d.level] || d.level

    tooltip.html(`
      <div><strong>${d.id}</strong></div>
      <div>描述：${d.description || '无'}</div>
      <div>效果：${effectText}</div>
      <div>优先级：${d.priority}</div>
      <div>层级：${levelText}</div>
    `)
    tooltip.style('opacity', 1)
  })

  node.on('mousemove', function(event) {
    tooltip
      .style('left', (event.pageX + 10) + 'px')
      .style('top', (event.pageY - 10) + 'px')
  })

  node.on('mouseout', function() {
    d3.select(this).select('circle')
      .transition()
      .duration(200)
      .attr('r', 18)

    tooltip.style('opacity', 0)
  })

  node.on('click', function(event, d) {
    emit('node-click', d)
  })

  link.on('mouseover', function(event, d) {
    d3.select(this)
      .transition()
      .duration(200)
      .attr('stroke-width', 4)
      .attr('stroke-opacity', 1)

    tooltip.html(`
      <div><strong>关系类型：${typeText(d.type)}</strong></div>
      <div>${d.desc || ''}</div>
    `)
    tooltip.style('opacity', 1)
  })

  link.on('mousemove', function(event) {
    tooltip
      .style('left', (event.pageX + 10) + 'px')
      .style('top', (event.pageY - 10) + 'px')
  })

  link.on('mouseout', function() {
    d3.select(this)
      .transition()
      .duration(200)
      .attr('stroke-width', 2)
      .attr('stroke-opacity', 0.6)

    tooltip.style('opacity', 0)
  })

  simulation = d3.forceSimulation(props.nodes)
    .force('link', d3.forceLink(props.edges).id(d => d.id).distance(120))
    .force('charge', d3.forceManyBody().strength(-300))
    .force('center', d3.forceCenter(width / 2, height / 2))
    .force('collision', d3.forceCollide().radius(40))

  simulation.on('tick', () => {
    link
      .attr('x1', d => d.source.x)
      .attr('y1', d => d.source.y)
      .attr('x2', d => d.target.x)
      .attr('y2', d => d.target.y)

    node.attr('transform', d => `translate(${d.x},${d.y})`)
  })

  function dragstarted(event, d) {
    if (!event.active) simulation.alphaTarget(0.3).restart()
    d.fx = d.x
    d.fy = d.y
  }

  function dragged(event, d) {
    d.fx = event.x
    d.fy = event.y
  }

  function dragended(event, d) {
    if (!event.active) simulation.alphaTarget(0)
    d.fx = null
    d.fy = null
  }
}

const typeText = (type) => {
  const map = {
    conflict: '冲突',
    override: '覆盖',
    complement: '互补'
  }
  return map[type] || type
}

onMounted(() => {
  if (props.nodes.length > 0) {
    initGraph()
  }
})

watch(() => [props.nodes, props.edges], () => {
  if (props.nodes.length > 0) {
    initGraph()
  }
}, { deep: true })

onUnmounted(() => {
  if (simulation) {
    simulation.stop()
  }
  d3.selectAll('.graph-tooltip').remove()
})
</script>

<style scoped>
.force-graph-container {
  width: 100%;
  height: 100%;
  position: relative;
  background: #f9fafb;
  border-radius: 8px;
  overflow: hidden;
}

.force-svg {
  display: block;
}

.graph-legend {
  position: absolute;
  top: 16px;
  right: 16px;
  background: rgba(255, 255, 255, 0.95);
  padding: 12px 16px;
  border-radius: 8px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
  font-size: 12px;
}

.legend-item {
  display: flex;
  align-items: center;
  margin-bottom: 6px;
}

.legend-item:last-child {
  margin-bottom: 0;
}

.legend-color {
  width: 12px;
  height: 12px;
  border-radius: 50%;
  margin-right: 8px;
}
</style>
