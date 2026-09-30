import type { Channel, ChannelModelPricing } from '@/api/admin/channels'
import type { ModelServicePolicy } from '@/types/modelService'

export function sameModel(left: string, right: string): boolean {
  return left.toLowerCase() === right.toLowerCase()
}

export function modelPolicy(channel: Channel, model: string, platform: string): ModelServicePolicy | undefined {
  return (channel.features_config?.model_services as ModelServicePolicy[] | undefined)?.find(
    policy => policy.platform === platform && sameModel(policy.model, model)
  )
}

export function modelPricing(channel: Channel, model: string, platform: string): ChannelModelPricing | undefined {
  return channel.model_pricing.find(entry => entry.platform === platform && entry.models.some(name => sameModel(name, model)))
}

export function wildcardPricing(channel: Channel, model: string, platform: string): ChannelModelPricing | undefined {
  return channel.model_pricing.find(entry => entry.platform === platform && entry.models.some(name =>
    name.endsWith('*') && model.toLowerCase().startsWith(name.slice(0, -1).toLowerCase())
  ))
}

// Update only the requested model. A shared price record is split so the other
// models keep their previous price, intervals and identity.
export function mergeModelPricing(
  entries: ChannelModelPricing[], model: string, platform: string, next: ChannelModelPricing | null
): ChannelModelPricing[] {
  const result = entries.flatMap(entry => {
    if (entry.platform !== platform || !entry.models.some(name => sameModel(name, model))) return [entry]
    const remaining = entry.models.filter(name => !sameModel(name, model))
    return remaining.length ? [{ ...entry, models: remaining }] : []
  })
  if (next === null) return result
  const replacement = { ...next, platform, models: [model] }
  delete replacement.id
  return [...result, replacement]
}

export function configuredModelOpen(channel: Channel | undefined, model: string, platform: string, groupID: number, upstreamModel = model): boolean {
  if (!channel) return true
  const allowedPolicy = (policy: ModelServicePolicy) => channel.status === 'active' && policy.state === 'published' &&
    (policy.groups == null || !!policy.groups[groupID]?.enabled)
  const requestedPolicy = modelPolicy(channel, model, platform)
  if (requestedPolicy) return allowedPolicy(requestedPolicy)
  // Disabled legacy channels do not enforce their pricing restrictions.
  if (channel.status !== 'active' || !channel.restrict_models) return true
  const mappings = Object.entries(channel.model_mapping?.[platform] || {})
  const exact = mappings.find(([source]) => sameModel(source, model))
  const wildcard = mappings.filter(([source]) => source.endsWith('*') && model.toLowerCase().startsWith(source.slice(0, -1).toLowerCase()))
    .sort((a, b) => b[0].length - a[0].length)[0]
  const billingModel = channel.billing_model_source === 'upstream' ? upstreamModel
    : channel.billing_model_source === 'requested' ? model : (exact || wildcard)?.[1] || model
  const billingPolicy = modelPolicy(channel, billingModel, platform)
  return billingPolicy ? allowedPolicy(billingPolicy) : !!modelPricing(channel, billingModel, platform) || !!wildcardPricing(channel, billingModel, platform)
}

export function mergeModelPolicy(channel: Channel, next: ModelServicePolicy): Record<string, unknown> {
  const policies = (channel.features_config?.model_services as ModelServicePolicy[] | undefined) || []
  return {
    ...channel.features_config,
    model_services: [
      ...policies.filter(policy => policy.platform !== next.platform || !sameModel(policy.model, next.model)),
      next
    ]
  }
}

export function modelScopeSnapshot(channel: Channel, model: string, platform: string): string {
  return JSON.stringify({
    groups: [...channel.group_ids].sort((a, b) => a - b),
    status: channel.status,
    policy: modelPolicy(channel, model, platform),
    pricing: modelPricing(channel, model, platform),
    wildcard: wildcardPricing(channel, model, platform)
  })
}
