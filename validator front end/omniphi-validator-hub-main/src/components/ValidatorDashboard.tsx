import { useState, useEffect } from 'react';
import { Card } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Alert, AlertDescription } from '@/components/ui/alert';
import {
  Activity,
  CheckCircle2,
  Clock,
  ExternalLink,
  RefreshCw,
  Settings,
  StopCircle,
  Play,
  Edit,
  TrendingUp,
  AlertCircle
} from 'lucide-react';
import { useValidatorStore } from '@/store/validatorStore';
import { useValidatorStatus } from '@/hooks/useValidatorStatus';
import { validatorApi } from '@/lib/api';
import { toast } from 'sonner';
import { LoadingSpinner } from './Common/LoadingSpinner';
import { ErrorBanner } from './Common/ErrorBanner';

export const ValidatorDashboard = () => {
  const { validatorConfig, validatorMode, validatorStatus, walletAddress, validatorInfo, setupRequestId } = useValidatorStore();
  const [operatorAddress, setOperatorAddress] = useState<string | undefined>();
  const [isActionLoading, setIsActionLoading] = useState(false);

  // Convert wallet address to operator address
  useEffect(() => {
    if (walletAddress) {
      setOperatorAddress(walletAddress.replace('omniphi', 'omniphivaloper'));
    }
  }, [walletAddress]);

  const { validator, status, loading, error, refresh } = useValidatorStatus(
    operatorAddress,
    validatorInfo?.nodeEndpoint
  );

  // Mock data fallback
  const uptimePercentage = 99.87;
  const totalRewards = "1,234.56";
  const dailyRewards = "12.34";

  const handleStopValidator = async () => {
    if (!setupRequestId) {
      toast.error('No setup request ID found. Cannot stop validator.');
      return;
    }

    if (validatorMode !== 'cloud') {
      toast.error('Only cloud validators can be stopped via dashboard. For local validators, stop the desktop app.');
      return;
    }

    setIsActionLoading(true);
    try {
      await validatorApi.stopValidator(setupRequestId);
      toast.success('Validator stopped successfully');
      refresh();
    } catch (error: any) {
      const errorMessage = error?.response?.data?.detail || error?.message || 'Failed to stop validator';
      toast.error(errorMessage);
    } finally {
      setIsActionLoading(false);
    }
  };

  const handleRedeployValidator = async () => {
    if (!setupRequestId) {
      toast.error('No setup request ID found. Cannot redeploy validator.');
      return;
    }

    if (validatorMode !== 'cloud') {
      toast.error('Only cloud validators can be redeployed via dashboard.');
      return;
    }

    setIsActionLoading(true);
    try {
      await validatorApi.redeployValidator(setupRequestId);
      toast.success('Validator redeployment initiated. Monitor provisioning status.');
      refresh();
    } catch (error: any) {
      const errorMessage = error?.response?.data?.detail || error?.message || 'Failed to redeploy validator';
      toast.error(errorMessage);
    } finally {
      setIsActionLoading(false);
    }
  };

  const handleEditMetadata = () => {
    toast.info('Edit metadata requires MsgEditValidator transaction. Integration coming soon.');
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'active':
        return 'bg-green-500';
      case 'provisioning':
        return 'bg-yellow-500 animate-pulse';
      case 'error':
        return 'bg-red-500';
      default:
        return 'bg-muted';
    }
  };

  const getStatusLabel = (status: string) => {
    switch (status) {
      case 'active':
        return 'Active';
      case 'provisioning':
        return 'Provisioning';
      case 'awaiting_signature':
        return 'Awaiting Signature';
      case 'error':
        return 'Error';
      default:
        return 'Unknown';
    }
  };

  if (loading && !validator) {
    return <LoadingSpinner size="lg" text="Loading validator data..." className="min-h-[400px]" />;
  }

  return (
    <div className="max-w-7xl mx-auto space-y-6 animate-fade-in">
      {/* Header */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div>
          <h2 className="text-3xl font-bold">{validatorConfig?.moniker || 'Validator Dashboard'}</h2>
          <p className="text-muted-foreground">Monitor and manage your validator</p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" size="sm" onClick={handleEditMetadata} disabled={isActionLoading}>
            <Edit className="h-4 w-4 mr-2" />
            Edit
          </Button>
          <Button variant="outline" size="sm" onClick={handleRedeployValidator} disabled={isActionLoading || validatorMode !== 'cloud'}>
            <Play className="h-4 w-4 mr-2" />
            Redeploy
          </Button>
          <Button variant="outline" size="sm" onClick={handleStopValidator} disabled={isActionLoading || validatorMode !== 'cloud'}>
            <StopCircle className="h-4 w-4 mr-2" />
            Stop
          </Button>
          <Button variant="outline" size="icon" onClick={refresh} disabled={loading || isActionLoading}>
            <RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
          </Button>
        </div>
      </div>

      {/* Error Banner */}
      {error && <ErrorBanner message={error} />}

      {/* Awaiting Signature Alert */}
      {validatorStatus === 'awaiting_signature' && (
        <Alert>
          <AlertCircle className="h-4 w-4" />
          <AlertDescription>
            Your validator node is ready! Please complete the setup by signing the transaction.
            <Button size="sm" className="ml-4" onClick={() => toast.info('Navigate to wizard')}>
              Sign Transaction
            </Button>
          </AlertDescription>
        </Alert>
      )}

      {/* Status Cards */}
      <div className="grid md:grid-cols-4 gap-6">
        <Card className="glass-card p-6">
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <h3 className="text-sm font-medium text-muted-foreground">Status</h3>
              <div className={`h-3 w-3 rounded-full ${getStatusColor(validatorStatus)}`} />
            </div>
            <div className="space-y-1">
              <p className="text-2xl font-bold">{getStatusLabel(validatorStatus)}</p>
              <p className="text-xs text-muted-foreground">Validator operational status</p>
            </div>
          </div>
        </Card>

        <Card className="glass-card p-6">
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <h3 className="text-sm font-medium text-muted-foreground">Uptime</h3>
              <Activity className="h-4 w-4 text-primary" />
            </div>
            <div className="space-y-1">
              <p className="text-2xl font-bold">{uptimePercentage}%</p>
              <p className="text-xs text-muted-foreground">Last 10,000 blocks</p>
            </div>
          </div>
        </Card>

        <Card className="glass-card p-6">
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <h3 className="text-sm font-medium text-muted-foreground">Commission</h3>
              <CheckCircle2 className="h-4 w-4 text-accent" />
            </div>
            <div className="space-y-1">
              <p className="text-2xl font-bold">
                {validator 
                  ? (parseFloat(validator.commission.commissionRates.rate) * 100).toFixed(2)
                  : validatorConfig 
                    ? (parseFloat(validatorConfig.commission.rate) * 100).toFixed(2) 
                    : '0'}%
              </p>
              <p className="text-xs text-muted-foreground">Current commission rate</p>
            </div>
          </div>
        </Card>

        <Card className="glass-card p-6">
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <h3 className="text-sm font-medium text-muted-foreground">Rewards</h3>
              <TrendingUp className="h-4 w-4 text-green-500" />
            </div>
            <div className="space-y-1">
              <p className="text-2xl font-bold">{totalRewards} OMNI</p>
              <p className="text-xs text-muted-foreground">+{dailyRewards} OMNI today</p>
            </div>
          </div>
        </Card>
      </div>

      {/* Main Info */}
      <div className="grid md:grid-cols-2 gap-6">
        <Card className="glass-card p-6 space-y-4">
          <h3 className="text-lg font-bold">Validator Details</h3>
          
          <div className="space-y-3 text-sm">
            <div className="flex justify-between">
              <span className="text-muted-foreground">Wallet Address</span>
              <span className="font-mono">{walletAddress?.slice(0, 10)}...{walletAddress?.slice(-6)}</span>
            </div>
            
            {validatorConfig?.website && (
              <div className="flex justify-between items-center">
                <span className="text-muted-foreground">Website</span>
                <a 
                  href={validatorConfig.website} 
                  target="_blank" 
                  rel="noopener noreferrer"
                  className="flex items-center gap-1 text-primary hover:underline"
                >
                  Visit <ExternalLink className="h-3 w-3" />
                </a>
              </div>
            )}

            {validatorConfig?.securityContact && (
              <div className="flex justify-between">
                <span className="text-muted-foreground">Security Contact</span>
                <span>{validatorConfig.securityContact}</span>
              </div>
            )}

            <div className="flex justify-between">
              <span className="text-muted-foreground">Min Self Delegation</span>
              <span>{validatorConfig?.minSelfDelegation || '0'}</span>
            </div>
          </div>

          {validatorConfig?.details && (
            <div className="pt-3 border-t border-border">
              <p className="text-sm text-muted-foreground">{validatorConfig.details}</p>
            </div>
          )}
        </Card>

        <Card className="glass-card p-6 space-y-4">
          <h3 className="text-lg font-bold">Recent Activity</h3>
          
          <div className="space-y-3">
            <div className="flex items-start gap-3 p-3 rounded-lg bg-card/50">
              <Clock className="h-4 w-4 text-primary mt-0.5" />
              <div className="flex-1 space-y-1">
                <p className="text-sm font-medium">Validator Deployed</p>
                <p className="text-xs text-muted-foreground">
                  {validatorMode === 'cloud' ? 'Cloud infrastructure' : 'Local setup'} initialized
                </p>
              </div>
              <Badge variant="secondary">Just now</Badge>
            </div>

            {validatorStatus === 'awaiting_signature' && (
              <div className="flex items-start gap-3 p-3 rounded-lg bg-accent/10 border border-accent/20">
                <Activity className="h-4 w-4 text-accent mt-0.5" />
                <div className="flex-1 space-y-1">
                  <p className="text-sm font-medium">Action Required</p>
                  <p className="text-xs text-muted-foreground">
                    Sign MsgCreateValidator transaction in your wallet
                  </p>
                </div>
              </div>
            )}
          </div>
        </Card>
      </div>

      {/* Action Buttons */}
      {validatorStatus === 'awaiting_signature' && (
        <Card className="glass-card p-6">
          <div className="flex items-center justify-between">
            <div>
              <h3 className="text-lg font-bold">Complete Validator Setup</h3>
              <p className="text-sm text-muted-foreground">
                Sign the transaction to activate your validator on the network
              </p>
            </div>
            <Button size="lg" className="glow-primary">
              Sign Transaction
            </Button>
          </div>
        </Card>
      )}
    </div>
  );
};
