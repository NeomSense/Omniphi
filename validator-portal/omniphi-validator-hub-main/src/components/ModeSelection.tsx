import { Card } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Cloud, Server, Check } from 'lucide-react';
import { ValidatorMode } from '@/types/validator';
import { useValidatorStore } from '@/store/validatorStore';

interface ModeSelectionProps {
  onModeSelected: () => void;
}

export const ModeSelection = ({ onModeSelected }: ModeSelectionProps) => {
  const { validatorMode, setValidatorMode } = useValidatorStore();

  const handleModeSelect = (mode: ValidatorMode) => {
    setValidatorMode(mode);
  };

  const handleContinue = () => {
    if (validatorMode) {
      onModeSelected();
    }
  };

  return (
    <div className="max-w-4xl mx-auto space-y-8 animate-fade-in">
      <div className="text-center space-y-4">
        <h2 className="text-3xl font-bold">Choose Your Validator Mode</h2>
        <p className="text-muted-foreground text-lg">
          Select how you want to run your validator node
        </p>
      </div>

      <div className="grid md:grid-cols-2 gap-6">
        <Card
          className={`p-6 cursor-pointer transition-all hover:scale-105 ${
            validatorMode === 'cloud'
              ? 'ring-2 ring-primary glass-card glow-primary'
              : 'glass-card hover:border-primary/50'
          }`}
          onClick={() => handleModeSelect('cloud')}
        >
          <div className="space-y-4">
            <div className="flex items-start justify-between">
              <div className="p-3 rounded-lg bg-primary/10">
                <Cloud className="h-8 w-8 text-primary" />
              </div>
              {validatorMode === 'cloud' && (
                <Check className="h-6 w-6 text-primary" />
              )}
            </div>
            
            <div className="space-y-2">
              <h3 className="text-xl font-bold">Cloud Validator</h3>
              <p className="text-muted-foreground">
                We handle all infrastructure. Your validator runs in our secure cloud environment.
              </p>
            </div>

            <ul className="space-y-2 text-sm text-muted-foreground">
              <li className="flex items-center gap-2">
                <Check className="h-4 w-4 text-accent" />
                Automatic setup & maintenance
              </li>
              <li className="flex items-center gap-2">
                <Check className="h-4 w-4 text-accent" />
                99.9% uptime guarantee
              </li>
              <li className="flex items-center gap-2">
                <Check className="h-4 w-4 text-accent" />
                No technical knowledge required
              </li>
            </ul>
          </div>
        </Card>

        <Card
          className={`p-6 cursor-pointer transition-all hover:scale-105 ${
            validatorMode === 'local'
              ? 'ring-2 ring-primary glass-card glow-primary'
              : 'glass-card hover:border-primary/50'
          }`}
          onClick={() => handleModeSelect('local')}
        >
          <div className="space-y-4">
            <div className="flex items-start justify-between">
              <div className="p-3 rounded-lg bg-primary/10">
                <Server className="h-8 w-8 text-primary" />
              </div>
              {validatorMode === 'local' && (
                <Check className="h-6 w-6 text-primary" />
              )}
            </div>
            
            <div className="space-y-2">
              <h3 className="text-xl font-bold">Local Validator</h3>
              <p className="text-muted-foreground">
                Run your validator on your own infrastructure with full control.
              </p>
            </div>

            <ul className="space-y-2 text-sm text-muted-foreground">
              <li className="flex items-center gap-2">
                <Check className="h-4 w-4 text-accent" />
                Complete control over your node
              </li>
              <li className="flex items-center gap-2">
                <Check className="h-4 w-4 text-accent" />
                Lower operating costs
              </li>
              <li className="flex items-center gap-2">
                <Check className="h-4 w-4 text-accent" />
                Technical expertise recommended
              </li>
            </ul>
          </div>
        </Card>
      </div>

      <div className="flex justify-center">
        <Button
          size="lg"
          onClick={handleContinue}
          disabled={!validatorMode}
          className="min-w-[200px]"
        >
          Continue
        </Button>
      </div>
    </div>
  );
};
