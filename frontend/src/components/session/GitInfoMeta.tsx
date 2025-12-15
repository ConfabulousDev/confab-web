import type { GitInfo } from '@/types';
import { formatRepoName } from '@/utils/formatting';
import { getRepoWebURL } from '@/utils/git';
import MetaItem from './MetaItem';

// SVG Icons
const RepoIcon = (
  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <path d="M9 19c-5 1.5-5-2.5-7-3m14 6v-3.87a3.37 3.37 0 0 0-.94-2.61c3.14-.35 6.44-1.54 6.44-7A5.44 5.44 0 0 0 20 4.77 5.07 5.07 0 0 0 19.91 1S18.73.65 16 2.48a13.38 13.38 0 0 0-7 0C6.27.65 5.09 1 5.09 1A5.07 5.07 0 0 0 5 4.77a5.44 5.44 0 0 0-1.5 3.78c0 5.42 3.3 6.61 6.44 7A3.37 3.37 0 0 0 9 18.13V22" />
  </svg>
);

const BranchIcon = (
  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <line x1="6" y1="3" x2="6" y2="15" />
    <circle cx="18" cy="6" r="3" />
    <circle cx="6" cy="18" r="3" />
    <path d="M18 9a9 9 0 0 1-9 9" />
  </svg>
);

const CommitIcon = (
  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <circle cx="12" cy="12" r="4" />
    <line x1="1.05" y1="12" x2="7" y2="12" />
    <line x1="17.01" y1="12" x2="22.96" y2="12" />
  </svg>
);

interface GitInfoMetaProps {
  gitInfo: GitInfo | null | undefined;
}

function GitInfoMeta({ gitInfo }: GitInfoMetaProps) {
  if (!gitInfo) return null;

  return (
    <>
      {gitInfo.repo_url && (
        <MetaItem
          icon={RepoIcon}
          value={formatRepoName(gitInfo.repo_url)}
          href={getRepoWebURL(gitInfo.repo_url) ?? undefined}
        />
      )}
      {gitInfo.branch && (
        <MetaItem icon={BranchIcon} value={gitInfo.branch} />
      )}
      {gitInfo.commit_sha && (
        <MetaItem icon={CommitIcon} value={gitInfo.commit_sha.substring(0, 7)} />
      )}
    </>
  );
}

export default GitInfoMeta;
