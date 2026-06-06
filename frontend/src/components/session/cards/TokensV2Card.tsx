import { CardWrapper, StatRow, CardLoading, CardError, SectionHeader } from './Card';
import { formatTokenCount, formatCost } from '@/utils/tokenStats';
import {
  TokenIcon,
  DollarIcon,
  ArrowRightIcon,
  ArrowLeftIcon,
  DiamondOutlineIcon,
  DiamondFilledIcon,
} from '@/components/icons';
import type { CardProps } from './types';

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
  const providerEntries = Object.entries(data.by_provider);

  return (
    <CardWrapper title="Tokens" icon={TokenIcon}>
      <StatRow
        label="Estimated cost"
        value={formatCost(totalCost)}
        icon={DollarIcon}
      />
      <StatRow
        label="Input"
        value={formatTokenCount(data.total_input)}
        icon={ArrowRightIcon}
      />
      <StatRow
        label="Output"
        value={formatTokenCount(data.total_output)}
        icon={ArrowLeftIcon}
      />
      {providerEntries.map(([providerName, provider]) => (
        <div key={providerName}>
          <SectionHeader label={providerName} />
          <StatRow
            label="Cost"
            value={formatCost(parseFloat(provider.cost_usd))}
            icon={DollarIcon}
          />
          {Object.entries(provider.models).map(([modelName, model]) => (
            <div key={modelName}>
              <SectionHeader label={modelName} />
              <StatRow
                label="Input"
                value={formatTokenCount(model.input)}
                icon={ArrowRightIcon}
              />
              <StatRow
                label="Output"
                value={formatTokenCount(model.output)}
                icon={ArrowLeftIcon}
              />
              {model.cache_read > 0 && (
                <StatRow
                  label="Cache read"
                  value={formatTokenCount(model.cache_read)}
                  icon={DiamondFilledIcon}
                />
              )}
              {model.cache_write > 0 && (
                <StatRow
                  label="Cache write"
                  value={formatTokenCount(model.cache_write)}
                  icon={DiamondOutlineIcon}
                />
              )}
              {model.reasoning > 0 && (
                <StatRow
                  label="Reasoning"
                  value={formatTokenCount(model.reasoning)}
                />
              )}
              <StatRow
                label="Cost"
                value={formatCost(parseFloat(model.cost_usd))}
                icon={DollarIcon}
              />
            </div>
          ))}
        </div>
      ))}
    </CardWrapper>
  );
}
