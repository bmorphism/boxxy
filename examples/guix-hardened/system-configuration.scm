(use-modules (gnu)
             (gnu services)
             (gnu services base)
             (gnu services shepherd)
             (gnu system nss)
             (guix gexp))

(use-service-modules networking ssh)
(use-package-modules bootloaders certs base julia)

(operating-system
  (host-name "openclaw-node-alpha")
  (timezone "UTC")
  (locale "en_US.utf8")
  (kernel-arguments '("console=hvc0" "root=/dev/vda1" "quiet"))
  (bootloader (bootloader-configuration
                (bootloader grub-bootloader)
                (targets '("/dev/vda"))))
  (file-systems (cons* (file-system
                         (device (file-system-label "my-root"))
                         (mount-point "/")
                         (type "ext4"))
                       (file-system
                         (device "boxxy-store")
                         (mount-point "/gnu/store")
                         (type "virtiofs")
                         (check? #f)
                         (mount-may-fail? #t)
                         (flags '(read-only)))
                       %base-file-systems))
  (users (cons (user-account
                (name "claw")
                (group "users")
                (supplementary-groups '("wheel"))
                (home-directory "/home/claw"))
               %base-user-accounts))
  (services
   (append (list
            (simple-service 'active-inference-service
                            shepherd-root-service-type
                            (list
                             (shepherd-service
                              (documentation "Run the OpenClaw Active Inference loop.")
                              (provision '(active-inference))
                              (requirement '(user-processes))
                              (start #~(make-forkexec-constructor
                                        (list #$(file-append julia "/bin/julia")
                                              "--project=/home/claw/openclaw"
                                              "-e" "using OpenClaw; run_inference_loop()")))
                              (stop #~(make-kill-destructor))))))
           (remove (lambda (service)
                     (let ((kind (service-kind service)))
                       (or (eq? kind avahi-service-type)
                           (eq? kind ntp-service-type)
                           (eq? kind network-manager-service-type)
                           (eq? kind openssh-service-type))))
                   %base-services)))
  (name-service-switch %mdns-host-lookup-nss))
