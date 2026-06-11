import { Fragment } from 'react';
import { CardWrapper, StatRow, CardLoading, CardError, SectionHeader } from './Card';
import { formatTokenCount, formatCost } from '@/utils/tokenStats';
import { formatModelKey } from '@/utils/formatting';
import { providerLabel } from '@/utils/providers';
import {
  TokenIcon,
  DollarIcon,
  ArrowRightIcon,
  ArrowLeftIcon,
  DiamondOutlineIcon,
  DiamondFilledIcon,
} from '@/components/icons';
import type { CardProps } from './types';
import styles from '../SessionSummaryPanel.module.css';

export type TokensV2Model = {
  input: number;
  output: number;
  cache_read: number;
  cache_write: number;
  reasoning: number;
  cost_usd: string;
};

export type TokensV2Provider = {
  cost_usd: string;
  models: Record<string, TokensV2Model>;
};

export type TokensV2CardData = {
  total_cost_usd: string;
  total_input: number;
  total_output: number;
  by_provider: Record<string, TokensV2Provider>;
};

const ZERO_COST_TOOLTIP =
  'Cost unavailable — session may use models not yet in the pricing table';

function ModelSection({ modelKey, model }: { modelKey: string; model: TokensV2Model }) {
  return (
    <div>
      <SectionHeader label={formatModelKey(modelKey)} />
      <StatRow label="Input" value={formatTokenCount(model.input)} icon={ArrowRightIcon} />
      <StatRow label="Output" value={formatTokenCount(model.output)} icon={ArrowLeftIcon} />
      {model.cache_read > 0 && (
        <StatRow label="Cache read" value={formatTokenCount(model.cache_read)} icon={DiamondFilledIcon} />
      )}
      {model.cache_write > 0 && (
        <StatRow label="Cache write" value={formatTokenCount(model.cache_write)} icon={DiamondOutlineIcon} />
      )}
      {model.reasoning > 0 && (
        <StatRow label="Reasoning" value={formatTokenCount(model.reasoning)} />
      )}
      <StatRow label="Cost" value={formatCost(parseFloat(model.cost_usd))} icon={DollarIcon} />
    </div>
  );
}

export function TokensV2Card({ data, loading, error }: CardProps<TokensV2CardData>) {
  if (error && !data) {
    return <CardError title="Tokens" error={error} icon={TokenIcon} />;
  }

  if (loading && !data) {
    return (
      <CardWrapper title="Tokens" icon={TokenIcon}>
        <CardLoading />
      </CardWrapper>
    );
  }

  if (!data) return null;

  const totalCost = parseFloat(data.total_cost_usd);
  const isZeroCost = totalCost === 0;
  const providerEntries = Object.entries(data.by_provider);
  // Single-provider sessions (Claude/Codex always; OpenCode single-vendor) drop
  // the redundant provider wrapper + per-provider cost row and render the model
  // sections directly under the totals. Multi-provider keeps the sections.
  const singleProvider = providerEntries.length === 1;

  return (
    <CardWrapper title="Tokens" icon={TokenIcon}>
      <StatRow
        label="Estimated cost"
        value={formatCost(totalCost)}
        icon={DollarIcon}
        tooltip={isZeroCost ? ZERO_COST_TOOLTIP : undefined}
        valueClassName={isZeroCost ? styles.costWarning : styles.cost}
      />
      <StatRow label="Input" value={formatTokenCount(data.total_input)} icon={ArrowRightIcon} />
      <StatRow label="Output" value={formatTokenCount(data.total_output)} icon={ArrowLeftIcon} />
      {providerEntries.map(([providerName, provider]) => {
        const modelSections = Object.entries(provider.models).map(([modelKey, model]) => (
          <ModelSection key={modelKey} modelKey={modelKey} model={model} />
        ));
        if (singleProvider) {
          return <Fragment key={providerName}>{modelSections}</Fragment>;
        }
        return (
          <div key={providerName}>
            <SectionHeader label={providerLabel(providerName)} />
            <StatRow label="Cost" value={formatCost(parseFloat(provider.cost_usd))} icon={DollarIcon} />
            {modelSections}
          </div>
        );
      })}
    </CardWrapper>
  );
}
