import { useEffect } from 'react';
import { Card } from '@/components/ui/card';
import { Progress } from '@/components/ui/progress';
import { CheckCircle2, Loader2, AlertCircle } from 'lucide-react';
import { useProvisioning } from '@/hooks/useProvisioning';
import { ErrorBanner } from './Common/ErrorBanner';

interface ProvisioningStepProps {
  setupId: string;
  onComplete: (consensusPubkey: string) => void;
}

export const ProvisioningStep = ({ setupId, onComplete }: ProvisioningStepProps) => {
  const { status, isComplete, error } = useProvisioning(setupId);

  useEffect(() => {
    if (isComplete && status.consensusPubkey) {
      onComplete(status.consensusPubkey);
    }
  }, [isComplete, status.consensusPubkey, onComplete]);

  const getStatusIcon = () => {
    if (error || status.status === 'failed') {
      return <AlertCircle className="h-5 w-5 text-destructive" />;
    }
    if (isComplete) {
      return <CheckCircle2 className="h-5 w-5 text-primary" />;
    }
    return <Loader2 className="h-5 w-5 animate-spin text-primary" />;
  };

  return (
    <Card className="glass-card p-8 space-y-6 animate-fade-in">
      <div className="space-y-2">
        <h3 className="text-2xl font-bold">Provisioning Validator Node</h3>
        <p className="text-muted-foreground">Setting up your validator infrastructure...</p>
      </div>

      {error && <ErrorBanner message={error} />}

      <div className="space-y-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            {getStatusIcon()}
            <span className="text-sm font-medium">{status.message}</span>
          </div>
          <span className="text-sm text-muted-foreground">{status.progress}%</span>
        </div>
        <Progress value={status.progress} className="h-2" />
      </div>

      <div className="rounded-lg bg-muted/30 p-4 font-mono text-sm space-y-2 max-h-64 overflow-y-auto">
        <div className="flex items-center gap-2">
          {getStatusIcon()}
          <span className={error ? 'text-destructive' : 'text-foreground'}>
            {status.message}
          </span>
        </div>
        
        {status.consensusPubkey && (
          <div className="mt-4 p-3 bg-primary/10 rounded border border-primary/20">
            <div className="text-xs text-muted-foreground mb-1">Consensus Public Key:</div>
            <div className="break-all text-xs text-primary">{status.consensusPubkey}</div>
          </div>
        )}
      </div>

      <div className="p-4 rounded-lg bg-accent/10 border border-accent/20 text-sm">
        <p className="text-muted-foreground">
          {isComplete
            ? 'Node provisioning complete! Proceeding to transaction signing...'
            : 'This process may take 5-10 minutes. Your node is being configured and will sync with the network.'}
        </p>
      </div>
    </Card>
  );
};
