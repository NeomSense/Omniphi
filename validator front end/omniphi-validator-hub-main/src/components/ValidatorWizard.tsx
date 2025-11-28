import { useState } from 'react';
import { Card } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { Progress } from '@/components/ui/progress';
import { ChevronLeft, ChevronRight, Loader2 } from 'lucide-react';
import { useValidatorStore } from '@/store/validatorStore';
import { ValidatorConfig } from '@/types/validator';
import { toast } from 'sonner';
import { validatorApi } from '@/lib/api';
import { ProvisioningStep } from './ProvisioningStep';
import { SignTransactionStep } from './SignTransactionStep';

const STEPS = [
  { id: 1, title: 'Basic Info', description: 'Validator identity' },
  { id: 2, title: 'Commission', description: 'Set your rates' },
  { id: 3, title: 'Confirm', description: 'Review and deploy' },
  { id: 4, title: 'Provision', description: 'Node setup' },
  { id: 5, title: 'Sign', description: 'Activate validator' },
];

export const ValidatorWizard = ({ onComplete }: { onComplete: () => void }) => {
  const { validatorMode, walletAddress, setValidatorConfig, setValidatorStatus, setSetupRequestId } = useValidatorStore();
  const [currentStep, setCurrentStep] = useState(0);
  const [isDeploying, setIsDeploying] = useState(false);
  const [setupId, setSetupId] = useState<string | null>(null);
  const [consensusPubkey, setConsensusPubkey] = useState<string | null>(null);

  const [config, setConfig] = useState<ValidatorConfig>({
    moniker: '',
    website: '',
    securityContact: '',
    details: '',
    commission: {
      rate: '0.10',
      maxRate: '0.20',
      maxChangeRate: '0.01',
    },
    minSelfDelegation: '1',
  });

  const progress = ((currentStep + 1) / STEPS.length) * 100;

  const handleNext = () => {
    if (currentStep === 0 && !config.moniker) {
      toast.error('Please enter a validator moniker');
      return;
    }
    if (currentStep < STEPS.length - 1) {
      setCurrentStep(currentStep + 1);
    }
  };

  const handleBack = () => {
    if (currentStep > 0) {
      setCurrentStep(currentStep - 1);
    }
  };

  const handleDeploy = async () => {
    if (!walletAddress) {
      toast.error('No wallet connected');
      return;
    }

    setIsDeploying(true);
    setValidatorStatus('provisioning');

    try {
      // Call appropriate API based on mode
      const result = validatorMode === 'cloud'
        ? await validatorApi.createCloudValidator(config, walletAddress)
        : await validatorApi.setupLocalValidator(config, walletAddress);

      setValidatorConfig(config);
      // Backend returns { setupRequest: { id: "uuid", ... } }
      const requestId = result.setupRequest?.id || result.id;
      if (!requestId) {
        throw new Error('No setup request ID received from backend');
      }
      setSetupId(requestId);
      setSetupRequestId(requestId); // Save to store for dashboard
      toast.success('Validator deployment initiated');
      setCurrentStep(3); // Move to provisioning step
    } catch (error: any) {
      console.error('Deployment error:', error);
      const errorMessage = error?.response?.data?.detail || error?.message || 'Failed to deploy validator';
      toast.error(errorMessage);
      setValidatorStatus('error');
    } finally {
      setIsDeploying(false);
    }
  };

  const handleProvisioningComplete = (pubkey: string) => {
    setConsensusPubkey(pubkey);
    setValidatorStatus('awaiting_signature');
    setCurrentStep(4); // Move to sign transaction step
  };

  const handleSignComplete = () => {
    setValidatorStatus('active');
    toast.success('Validator activated successfully!');
    onComplete();
  };

  return (
    <div className="max-w-3xl mx-auto space-y-6 animate-fade-in">
      {/* Progress */}
      <div className="space-y-3">
        <div className="flex justify-between items-center">
          {STEPS.map((step, idx) => (
            <div
              key={step.id}
              className={`flex-1 text-center ${
                idx <= currentStep ? 'text-primary' : 'text-muted-foreground'
              }`}
            >
              <div className="text-sm font-medium">{step.title}</div>
              <div className="text-xs">{step.description}</div>
            </div>
          ))}
        </div>
        <Progress value={progress} className="h-2" />
      </div>

      {/* Content */}
      {currentStep === 3 && setupId ? (
        <ProvisioningStep 
          setupId={setupId} 
          onComplete={handleProvisioningComplete}
        />
      ) : currentStep === 4 && consensusPubkey && walletAddress ? (
        <SignTransactionStep
          config={config}
          consensusPubkey={consensusPubkey}
          walletAddress={walletAddress}
          onComplete={handleSignComplete}
        />
      ) : (
        <Card className="glass-card p-8">
          {currentStep === 0 && (
          <div className="space-y-6">
            <h3 className="text-2xl font-bold">Basic Information</h3>
            
            <div className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="moniker">Validator Moniker *</Label>
                <Input
                  id="moniker"
                  placeholder="My Awesome Validator"
                  value={config.moniker}
                  onChange={(e) => setConfig({ ...config, moniker: e.target.value })}
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="website">Website</Label>
                <Input
                  id="website"
                  placeholder="https://myvalidator.com"
                  value={config.website}
                  onChange={(e) => setConfig({ ...config, website: e.target.value })}
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="security">Security Contact</Label>
                <Input
                  id="security"
                  placeholder="security@myvalidator.com"
                  value={config.securityContact}
                  onChange={(e) => setConfig({ ...config, securityContact: e.target.value })}
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="details">Details</Label>
                <Textarea
                  id="details"
                  placeholder="Tell the community about your validator..."
                  value={config.details}
                  onChange={(e) => setConfig({ ...config, details: e.target.value })}
                  rows={4}
                />
              </div>
            </div>
          </div>
        )}

        {currentStep === 1 && (
          <div className="space-y-6">
            <h3 className="text-2xl font-bold">Commission Settings</h3>
            
            <div className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="rate">Commission Rate (%)</Label>
                <Input
                  id="rate"
                  type="number"
                  step="0.01"
                  min="0"
                  max="1"
                  value={parseFloat(config.commission.rate) * 100}
                  onChange={(e) => setConfig({
                    ...config,
                    commission: { ...config.commission, rate: (parseFloat(e.target.value) / 100).toString() }
                  })}
                />
                <p className="text-sm text-muted-foreground">
                  The percentage you earn from delegations
                </p>
              </div>

              <div className="space-y-2">
                <Label htmlFor="maxRate">Max Commission Rate (%)</Label>
                <Input
                  id="maxRate"
                  type="number"
                  step="0.01"
                  min="0"
                  max="1"
                  value={parseFloat(config.commission.maxRate) * 100}
                  onChange={(e) => setConfig({
                    ...config,
                    commission: { ...config.commission, maxRate: (parseFloat(e.target.value) / 100).toString() }
                  })}
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="maxChangeRate">Max Change Rate (% per day)</Label>
                <Input
                  id="maxChangeRate"
                  type="number"
                  step="0.01"
                  min="0"
                  max="1"
                  value={parseFloat(config.commission.maxChangeRate) * 100}
                  onChange={(e) => setConfig({
                    ...config,
                    commission: { ...config.commission, maxChangeRate: (parseFloat(e.target.value) / 100).toString() }
                  })}
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="minSelfDelegation">Min Self Delegation</Label>
                <Input
                  id="minSelfDelegation"
                  type="number"
                  min="1"
                  value={config.minSelfDelegation}
                  onChange={(e) => setConfig({ ...config, minSelfDelegation: e.target.value })}
                />
              </div>
            </div>
          </div>
        )}

        {currentStep === 2 && (
          <div className="space-y-6">
            <h3 className="text-2xl font-bold">Review Configuration</h3>
            
            <div className="space-y-4 text-sm">
              <div className="grid grid-cols-2 gap-4">
                <div className="space-y-1">
                  <div className="text-muted-foreground">Mode</div>
                  <div className="font-medium capitalize">{validatorMode}</div>
                </div>
                <div className="space-y-1">
                  <div className="text-muted-foreground">Moniker</div>
                  <div className="font-medium">{config.moniker}</div>
                </div>
                <div className="space-y-1">
                  <div className="text-muted-foreground">Commission</div>
                  <div className="font-medium">{parseFloat(config.commission.rate) * 100}%</div>
                </div>
                <div className="space-y-1">
                  <div className="text-muted-foreground">Min Self Delegation</div>
                  <div className="font-medium">{config.minSelfDelegation}</div>
                </div>
              </div>

              {config.website && (
                <div className="space-y-1">
                  <div className="text-muted-foreground">Website</div>
                  <div className="font-medium">{config.website}</div>
                </div>
              )}

              {config.details && (
                <div className="space-y-1">
                  <div className="text-muted-foreground">Details</div>
                  <div className="font-medium">{config.details}</div>
                </div>
              )}
            </div>

            <div className="p-4 rounded-lg bg-accent/10 border border-accent/20">
              <p className="text-sm text-muted-foreground">
                After deployment, you'll receive a consensus public key. You'll need to sign a 
                MsgCreateValidator transaction with your connected wallet to activate your validator.
              </p>
            </div>
          </div>
        )}
        </Card>
      )}

      {/* Navigation */}
      {currentStep < 3 && (
        <div className="flex justify-between">
          <Button
            variant="outline"
            onClick={handleBack}
            disabled={currentStep === 0 || isDeploying}
          >
            <ChevronLeft className="mr-2 h-4 w-4" />
            Back
          </Button>

          {currentStep < 2 ? (
            <Button onClick={handleNext} disabled={isDeploying}>
              Next
              <ChevronRight className="ml-2 h-4 w-4" />
            </Button>
          ) : (
            <Button 
              onClick={handleDeploy} 
              disabled={isDeploying || !config.moniker}
              className="glow-primary"
            >
              {isDeploying && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              Deploy Validator
            </Button>
          )}
        </div>
      )}
    </div>
  );
};
