import Image from "next/image";
import Link from "next/link";

const asset = "/figma/register";

const highlights = [
  { icon: "sparkles.svg", label: "Автоматичний підбір взаємних збігів" },
  { icon: "calendar.svg", label: "Планування занять і нагадування" },
  { icon: "award.svg", label: "Рейтинг і відгуки після кожного обміну" },
];

export function AuthBrandPanel() {
  return (
    <aside className="register-brand" aria-label="Про SkillSwap">
      <Link className="register-brand__logo" href="/" aria-label="SkillSwap — на головну">
        <Image src={`${asset}/mark.svg`} alt="" width={30} height={30} />
        <span>SkillSwap</span>
      </Link>

      <div className="register-brand__story">
        <h2>
          Обмінюйся навичками,
          <br />а не грошима
        </h2>
        <p>
          SkillSwap знаходить студентів, які вміють те, що ти хочеш опанувати — і хочуть навчитися
          того, що вмієш ти.
        </p>
        <ul className="register-brand__highlights">
          {highlights.map(({ icon, label }) => (
            <li key={icon}>
              <span className="register-brand__icon">
                <Image src={`${asset}/${icon}`} alt="" width={16} height={16} />
              </span>
              <span>{label}</span>
            </li>
          ))}
        </ul>
      </div>

      <figure className="register-brand__quote">
        <blockquote>
          «За місяць навчила двох людей Photoshop і нарешті сама взяла в руки гітару. Без жодної
          гривні.»
        </blockquote>
        <figcaption>
          <span className="register-brand__avatar" aria-hidden="true">
            ОК
          </span>
          <span>
            <strong>Олена Ковальчук</strong>
            <small>КПІ · ФІОТ, 3 курс</small>
          </span>
        </figcaption>
      </figure>
    </aside>
  );
}
