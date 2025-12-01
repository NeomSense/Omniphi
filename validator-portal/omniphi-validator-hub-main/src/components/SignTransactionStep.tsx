import { useState, useEffect } from 'react';
import { Card } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { FileText, Send, Loader2, Copy, CheckCircle2 } from 'lucide-react';
import { ValidatorConfig } from '@/types/validator';
import { useTxBuilder } from '@/hooks/useTxBuilder';
import { useWallet } from '@/hooks/useWallet';
import { toast } from 'sonner';

interface SignTransactionStepProps {
  config: ValidatorConfig;
  consensusPubkey: string;
  walletAddress: string;
  onComplete: () => void;
}

export const SignTransactionStep = ({ config, consensusPubkey, walletAddress, onComplete }: SignTransactionStepProps) => {
  const [isSigning, setIsSigning] = useState(false);
  const [txJson, setTxJson] = useState('');
  const [copied, setCopied] = useState(false);
  const { buildMsgCreateValidator, buildTransaction } = useTxBuilder();
  const { signTransaction } = useWallet();

  useEffect(() => {
    const validatorAddress = walletAddress.replace('omniphi', 'omniphivaloper');

    const msg = buildMsgCreateValidator(
      config,
      consensusPubkey,
      walletAddress,
      validatorAddress,
      '1000000'
    );

    const tx = buildTransaction(
      [{
        '@type': '/cosmos.staking.v1beta1.MsgCreateValidator',
        ...msg,
      }],
      'Creating Omniphi Validator'
    );

    setTxJson(JSON.stringify(tx, null, 2));
  }, [config, consensusPubkey, walletAddress, buildMsgCreateValidator, buildTransaction]);

  const handleCopy = () => {
    navigator.clipboard.writeText(txJson);
    setCopied(true);
    toast.success('Transaction copied to clipboard');
    setTimeout(() => setCopied(false), 2000);
  };

  const handleSign = async () => {
    setIsSigning(true);

    try {
      const tx = JSON.parse(txJson);
      await signTransaction(tx);
      toast.success('Transaction signed successfully!');
      
      await new Promise(resolve => setTimeout(resolve, 1500));
      toast.success('Transaction broadcasted to network!');
      
      setTimeout(() => onComplete(), 1000);
    } catch (error: any) {
      console.error('Signing error:', error);
      toast.error(error.message || 'Failed to sign transaction');
    } finally {
      setIsSigning(false);
    }
  };

  return (
    <Card className="glass-card p-8 space-y-6 animate-fade-in">
      <div className="space-y-2">
        <h3 className="text-2xl font-bold">Sign Create Validator Transaction</h3>
        <p className="text-muted-foreground">
          Review and sign the transaction to activate your validator on the Omniphi network.
        </p>
      </div>

      <div className="space-y-4">
        <div className="flex items-center justify-between">
          <h4 className="text-sm font-semibold flex items-center gap-2">
            <FileText className="h-4 w-4" />
            Transaction JSON
          </h4>
          <Button variant="outline" size="sm" onClick={handleCopy} disabled={isSigning}>
            {copied ? (
              <>
                <CheckCircle2 className="mr-2 h-3 w-3" />
                Copied
              </>
            ) : (
              <>
                <Copy className="mr-2 h-3 w-3" />
                Copy
              </>
            )}
          </Button>
        </div>

        <pre className="bg-muted/30 p-4 rounded-lg overflow-x-auto text-xs font-mono max-h-96 border border-border">
          {txJson}
        </pre>
      </div>

      <div className="space-y-4">
        <div className="p-4 rounded-lg bg-accent/10 border border-accent/20">
          <h4 className="font-semibold mb-2">Transaction Summary</h4>
          <div className="space-y-2 text-sm">
            <div className="flex justify-between">
              <span className="text-muted-foreground">Type:</span>
              <span className="font-medium">MsgCreateValidator</span>
            </div>
            <div className="flex justify-between">
              <span className="text-muted-foreground">Moniker:</span>
              <span className="font-medium">{config.moniker}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-muted-foreground">Commission:</span>
              <span className="font-medium">{parseFloat(config.commission.rate) * 100}%</span>
            </div>
            <div className="flex justify-between">
              <span className="text-muted-foreground">Self Delegation:</span>
              <span className="font-medium">1 OMNI</span>
            </div>
          </div>
        </div>

        <Button onClick={handleSign} disabled={isSigning} className="w-full glow-primary" size="lg">
          {isSigning ? (
            <>
              <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              Signing Transaction...
            </>
          ) : (
            <>
              <Send className="mr-2 h-4 w-4" />
              Sign & Broadcast Transaction
            </>
          )}
        </Button>
      </div>

      <div className="p-4 rounded-lg bg-primary/10 border border-primary/20 text-sm">
        <p className="text-muted-foreground">
          Your wallet extension will prompt you to sign this transaction. Make sure you have sufficient balance
          for the self-delegation (1 OMNI) and transaction fees.
        </p>
      </div>
    </Card>
  );
};
